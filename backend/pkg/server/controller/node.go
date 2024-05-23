package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

var (
	nodePreHooks  = []preHook[*model.Node]{nodePreHookCheckCycle}
	nodePostHooks = []postHook[*model.Node]{nodePostHookCountAsset, nodePostHookHasChild}
	nodeDcs       = []deleteCheck{nodeDelHook}
)

// CreateNode godoc
//
//	@Tags		node
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node [post]
func (c *Controller) CreateNode(ctx *gin.Context) {
	doCreate(ctx, false, &model.Node{}, "")
}

// DeleteNode godoc
//
//	@Tags		node
//	@Param		id	path		int	true	"node id"
//	@Success	200	{object}	HttpResponse
//	@Router		/node/:id [delete]
func (c *Controller) DeleteNode(ctx *gin.Context) {
	doDelete(ctx, false, &model.Node{}, nodeDcs...)
}

// UpdateNode godoc
//
//	@Tags		node
//	@Param		id		path		int			true	"node id"
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node/:id [put]
func (c *Controller) UpdateNode(ctx *gin.Context) {
	doUpdate(ctx, false, &model.Node{}, nodePreHooks...)
}

// GetNodes godoc
//
//	@Tags		node
//	@Param		page_index		query		int		true	"node id"
//	@Param		page_size		query		int		true	"node id"
//	@Param		id				query		int		false	"node id"
//	@Param		ids				query		string	false	"node ids"
//	@Param		parent_id		query		int		false	"node's parent id"
//	@Param		name			query		string	false	"node name"
//	@Param		no_self_child	query		int		false	"exclude itself and its child"
//	@Param		self_parent		query		int		false	"include itself and its parent"
//	@Success	200				{object}	HttpResponse{data=ListData{list=[]model.Node}}
//	@Router		/node [get]
func (c *Controller) GetNodes(ctx *gin.Context) {
	db := mysql.DB.Model(&model.Node{})

	db = filterEqual(ctx, db, "id", "parent_id")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}
	if id, ok := ctx.GetQuery("no_self_child"); ok {
		ids, err := handleNoSelfChild(cast.ToInt(id))
		if err != nil {
			return
		}
		db = db.Where("id NOT IN ?", ids)
	}

	if id, ok := ctx.GetQuery("self_parent"); ok {
		ids, err := handleSelfParent(cast.ToInt(id))
		if err != nil {
			return
		}
		db = db.Where("id IN ?", ids)
	}

	db = db.Order("name DESC")

	doGet[*model.Node](ctx, false, db, "", nodePostHooks...)
}

func nodePreHookCheckCycle(ctx *gin.Context, data *model.Node) {
	nodes := make([]*model.NodeIdPid, 0)
	err := mysql.DB.Model(nodes).Find(&nodes).Error
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int) bool
	dfs = func(x int) bool {
		b := x == data.ParentId
		for _, y := range g[x] {
			b = b || dfs(y)
		}
		return b
	}

	if err != nil || dfs(cast.ToInt(ctx.Param("id"))) {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument})
	}
}

func nodePostHookCountAsset(ctx *gin.Context, data []*model.Node) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	isAdmin := acl.IsAdmin(currentUser)
	assets := make([]*model.AssetIdPid, 0)
	db := mysql.DB.Model(&model.Asset{})
	if !isAdmin {
		authorizationResourceIds, err := GetAutorizationResourceIds(ctx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		ids := make([]int, 0)
		if err = mysql.DB.
			Model(&model.Authorization{}).
			Where("resource_id IN ?", authorizationResourceIds).
			Distinct().
			Pluck("asset_id", &ids).
			Error; err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		db = db.Where("id IN ?", ids)
	}
	if err := db.Find(&assets).Error; err != nil {
		logger.L.Error("node posthookfailed asset count", zap.Error(err))
		return
	}
	nodes := make([]*model.NodeIdPid, 0)
	if err := mysql.DB.Model(nodes).Find(&nodes).Error; err != nil {
		logger.L.Error("node posthookfailed node", zap.Error(err))
		return
	}
	m := make(map[int]int64)
	for _, a := range assets {
		m[a.ParentId] += 1
	}
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int) int64
	dfs = func(x int) int64 {
		for _, y := range g[x] {
			m[x] += dfs(y)
		}
		return m[x]
	}
	dfs(0)

	for _, d := range data {
		d.AssetCount = m[d.Id]
	}
}

func nodePostHookHasChild(ctx *gin.Context, data []*model.Node) {
	ps := make([]int, 0)
	if err := mysql.DB.
		Model(&model.Node{}).
		Where("parent_id IN ?", lo.Map(data, func(n *model.Node, _ int) int { return n.Id })).
		Pluck("parent_id", &ps).
		Error; err != nil {
		logger.L.Error("node posthookfailed has child", zap.Error(err))
		return
	}
	pm := lo.SliceToMap(ps, func(pid int) (int, bool) { return pid, true })
	for _, n := range data {
		n.HasChild = pm[n.Id]
	}
}

func nodeDelHook(ctx *gin.Context, id int) {
	noChild := true
	noChild = noChild && errors.Is(mysql.DB.Model(&model.Node{}).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
	noChild = noChild && errors.Is(mysql.DB.Model(&model.Asset{}).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
	if noChild {
		return
	}

	err := &ApiError{Code: ErrHasChild, Data: nil}
	ctx.AbortWithError(http.StatusBadRequest, err)
}

func handleNoSelfChild(id int) (ids []int, err error) {
	nodes := make([]*model.NodeIdPid, 0)
	if err = mysql.DB.Model(nodes).Find(&nodes).Error; err != nil {
		return
	}
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int)
	dfs = func(x int) {
		ids = append(ids, x)
		for _, y := range g[x] {
			dfs(y)
		}
	}
	dfs(id)

	return
}

func handleSelfParent(id int) (ids []int, err error) {
	nodes := make([]*model.NodeIdPid, 0)
	if err = mysql.DB.Model(nodes).Find(&nodes).Error; err != nil {
		return
	}
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int)
	dfs = func(x int) {
		ids = append(ids, x)
		for _, y := range g[x] {
			dfs(y)
		}
	}
	dfs(id)

	ids = append(lo.Without(lo.Keys(g), ids...), id)

	return
}
