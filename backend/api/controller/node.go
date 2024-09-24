package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

const (
	kFmtAllNodes = "allNodes"
	kFmtNodeIds  = "assetIds-%d"
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
	redis.RC.Del(ctx, kFmtAllNodes)
	doCreate(ctx, false, &model.Node{}, "")
}

// DeleteNode godoc
//
//	@Tags		node
//	@Param		id	path		int	true	"node id"
//	@Success	200	{object}	HttpResponse
//	@Router		/node/:id [delete]
func (c *Controller) DeleteNode(ctx *gin.Context) {
	redis.RC.Del(ctx, kFmtAllNodes)
	doDelete(ctx, false, &model.Node{}, conf.RESOURCE_NODE, nodeDcs...)
}

// UpdateNode godoc
//
//	@Tags		node
//	@Param		id		path		int			true	"node id"
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node/:id [put]
func (c *Controller) UpdateNode(ctx *gin.Context) {
	redis.RC.Del(ctx, kFmtAllNodes)
	doUpdate(ctx, false, &model.Node{}, conf.RESOURCE_NODE, nodePreHooks...)
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
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	info := cast.ToBool(ctx.Query("info"))

	db := mysql.DB.Model(&model.Node{})

	db = filterEqual(ctx, db, "id", "parent_id")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}
	if id, ok := ctx.GetQuery("no_self_child"); ok {
		ids, err := handleNoSelfChild(ctx, cast.ToInt(id))
		if err != nil {
			return
		}
		db = db.Where("id NOT IN ?", ids)
	}

	if id, ok := ctx.GetQuery("self_parent"); ok {
		ids, err := handleSelfParent(ctx, cast.ToInt(id))
		if err != nil {
			return
		}
		db = db.Where("id IN ?", ids)
	}

	if info && !acl.IsAdmin(currentUser) {
		ids, err := GetNodeIdsByAuthorization(ctx)
		if err != nil {
			return
		}
		db = db.Where("id IN ?", ids)
	}

	db = db.Order("name DESC")

	doGet(ctx, !info, db, acl.GetResourceTypeName(conf.RESOURCE_NODE), nodePostHooks...)
}

func nodePreHookCheckCycle(ctx *gin.Context, data *model.Node) {
	nodes := make([]*model.Node, 0)
	err := mysql.DB.Model(&model.Node{}).Find(&nodes).Error
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
		authorizationResourceIds, err := getAutorizationResourceIds(ctx)
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
		logger.L().Error("node posthookfailed asset count", zap.Error(err))
		return
	}
	nodes := make([]*model.Node, 0)
	if err := mysql.DB.Model(&model.Node{}).Find(&nodes).Error; err != nil {
		logger.L().Error("node posthookfailed node", zap.Error(err))
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
		logger.L().Error("node posthookfailed has child", zap.Error(err))
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

func handleNoSelfChild(ctx context.Context, id int) (ids []int, err error) {
	nodes, err := getAllNodes(ctx)
	if err != nil {
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

func handleSelfParent(ctx context.Context, id int) (ids []int, err error) {
	nodes, err := getAllNodes(ctx)
	if err != nil {
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

func handleSelfChild(ctx context.Context, parentIds []int) (ids []int, err error) {
	nodes, err := getAllNodes(ctx)
	if err != nil {
		return
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int, bool)
	dfs = func(x int, b bool) {
		if b {
			ids = append(ids, x)
		}
		for _, y := range g[x] {
			dfs(y, b || lo.Contains(parentIds, x))
		}
	}
	dfs(0, false)

	return
}

func GetNodeIdsByAuthorization(ctx *gin.Context) (ids []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	authIds, err := getAuthorizationIds(ctx)
	if err != nil {
		return
	}
	ctx.Set(kAuthorizationIds, authIds)

	k := fmt.Sprintf(kFmtNodeIds, currentUser.GetUid())
	if err = redis.Get(ctx, k, &ids); err == nil {
		return
	}

	parentNodeIds, _, _ := getIdsByAuthorizationIds(ctx)
	ids, err = handleSelfChild(ctx, parentNodeIds)
	if err != nil {
		return
	}

	redis.SetEx(ctx, k, ids, time.Minute)

	return
}

func getAllNodes(ctx context.Context) (nodes []*model.Node, err error) {
	if err = redis.Get(ctx, kFmtAllNodes, &nodes); err != nil {
		if err = mysql.DB.Model(&model.Node{}).Find(&nodes).Error; err != nil {
			return
		}
		redis.SetEx(ctx, kFmtAllNodes, nodes, time.Hour)
	}

	return
}

func handleSelfChildPerms(ctx context.Context, id2perms map[int][]string) (res map[int][]string, err error) {
	nodes, err := getAllNodes(ctx)
	if err != nil {
		return
	}

	res = make(map[int][]string)
	id2rid := make(map[int]int)
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
		id2rid[n.Id] = n.ResourceId
		res[id2rid[n.Id]] = id2perms[id2rid[n.Id]]
	}
	var dfs func(int)
	dfs = func(x int) {
		for _, y := range g[x] {
			res[id2rid[y]] = lo.Uniq(append(res[id2rid[y]], res[id2rid[x]]...))
			dfs(y)
		}
	}
	dfs(0)

	return
}

func getNodeId2ResId(ctx context.Context) (resid2ids map[int]int, err error) {
	nodes, err := getAllNodes(ctx)
	if err != nil {
		return
	}

	return lo.SliceToMap(nodes, func(n *model.Node) (int, int) { return n.Id, n.ResourceId }), nil
}
