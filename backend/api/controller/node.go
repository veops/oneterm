package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/util"
)

const (
	kFmtAllNodes = "allNodes"
	kFmtNodeIds  = "nodeIds-%d"
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
	doCreate(ctx, true, &model.Node{}, conf.RESOURCE_NODE)
}

// DeleteNode godoc
//
//	@Tags		node
//	@Param		id	path		int	true	"node id"
//	@Success	200	{object}	HttpResponse
//	@Router		/node/:id [delete]
func (c *Controller) DeleteNode(ctx *gin.Context) {
	redis.RC.Del(ctx, kFmtAllNodes)
	doDelete(ctx, true, &model.Node{}, conf.RESOURCE_NODE, nodeDcs...)
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
	doUpdate(ctx, true, &model.Node{}, conf.RESOURCE_NODE, nodePreHooks...)
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

	db := mysql.DB.Model(model.DefaultNode)

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
		if ids, err = handleSelfParent(ctx, ids...); err != nil {
			return
		}
		db = db.Where("id IN ?", ids)
	}

	doGet(ctx, !info, db, conf.RESOURCE_NODE, nodePostHooks...)
}

func nodePreHookCheckCycle(ctx *gin.Context, data *model.Node) {
	nodes := make([]*model.Node, 0)
	err := mysql.DB.Model(model.DefaultNode).Find(&nodes).Error
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
	assets := make([]*model.AssetIdPid, 0)
	db := mysql.DB.Model(model.DefaultAsset)
	if !acl.IsAdmin(currentUser) {
		info := cast.ToBool(ctx.Query("info"))
		if info {
			assetIds, err := GetAssetIdsByAuthorization(ctx)
			if err != nil {
				return
			}
			db = db.Where("id IN ?", assetIds)
		} else {
			assetResId, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), conf.RESOURCE_ASSET)
			if err != nil {
				return
			}
			db, err = handleAssetIds(ctx, db, assetResId)
			if err != nil {
				return
			}
		}
	}
	if err := db.Find(&assets).Error; err != nil {
		logger.L().Error("node posthookfailed asset count", zap.Error(err))
		return
	}
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
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
	info := cast.ToBool(ctx.Query("info"))
	ps := make(map[int]bool, 0)
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	nodes, _ := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if acl.IsAdmin(currentUser) {
		ps = lo.SliceToMap(nodes, func(n *model.Node) (int, bool) { return n.ParentId, true })
	} else {
		assets, _ := util.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if info {
			assetIds, _ := GetAssetIdsByAuthorization(ctx)
			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })
			pids := lo.Map(assets, func(a *model.Asset, _ int) int { return a.ParentId })
			pids, _ = handleSelfParent(ctx, pids...)
			nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(pids, n.Id) })
			ps = lo.SliceToMap(nodes, func(a *model.Node) (int, bool) { return a.ParentId, true })
		} else {
			var assetResIds, nodeResIds, pids, nids []int
			eg := errgroup.Group{}
			eg.Go(func() (err error) {
				assetResIds, err = acl.GetRoleResourceIds(ctx, currentUser.GetRid(), conf.RESOURCE_ASSET)
				assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetResIds, a.ResourceId) })
				pids = lo.Map(assets, func(n *model.Asset, _ int) int { return n.ParentId })
				pids, _ = handleSelfParent(ctx, pids...)
				return
			})
			eg.Go(func() (err error) {
				nodeResIds, err = acl.GetRoleResourceIds(ctx, currentUser.GetRid(), conf.RESOURCE_NODE)
				ns := lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(nodeResIds, n.ResourceId) })
				nids, _ = handleSelfChild(ctx, lo.Map(ns, func(n *model.Node, _ int) int { return n.Id })...)
				nids, _ = handleSelfParent(ctx, nids...)
				return
			})
			eg.Wait()
			nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(pids, n.Id) || lo.Contains(nids, n.Id) })
			ps = lo.SliceToMap(nodes, func(n *model.Node) (int, bool) { return n.ParentId, true })
		}
	}
	for _, n := range data {
		n.HasChild = ps[n.Id]
	}
}

func nodeDelHook(ctx *gin.Context, id int) {
	noChild := true
	noChild = noChild && errors.Is(mysql.DB.Model(model.DefaultNode).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
	noChild = noChild && errors.Is(mysql.DB.Model(model.DefaultAsset).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
	if noChild {
		return
	}

	err := &ApiError{Code: ErrHasChild, Data: nil}
	ctx.AbortWithError(http.StatusBadRequest, err)
}

func handleNoSelfChild(ctx context.Context, ids ...int) (res []int, err error) {
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
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
			res = append(res, x)
		}
		for _, y := range g[x] {
			dfs(y, b || lo.Contains(ids, x))
		}
	}
	dfs(0, false)

	return
}

func handleNoSelfParent(ctx context.Context, ids ...int) (res []int, err error) {
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	t := make([]int, 0)
	var dfs func(int)
	dfs = func(x int) {
		if lo.Contains(ids, x) {
			res = append(res, t...)
		}
		t = append(t, x)
		for _, y := range g[x] {
			dfs(y)
		}
		t = t[:len(t)-1]
	}
	dfs(0)

	res = lo.Uniq(res)

	return
}

func handleSelfParent(ctx context.Context, ids ...int) (res []int, err error) {
	res, err = handleNoSelfParent(ctx, ids...)
	if err != nil {
		return
	}
	res = lo.Uniq(append(res, ids...))

	return
}

func handleSelfChild(ctx context.Context, ids ...int) (res []int, err error) {
	res, err = handleNoSelfChild(ctx, ids...)
	if err != nil {
		return
	}
	res = lo.Uniq(append(res, ids...))

	return
}

func GetNodeIdsByAuthorization(ctx *gin.Context) (ids []int, err error) {
	assetIds, err := GetAssetIdsByAuthorization(ctx)
	if err != nil {
		return
	}
	assets, _ := util.GetAllFromCacheDb(ctx, model.DefaultAsset)
	assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })
	ids = lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int { return a.ParentId }))

	return
}

func handleSelfChildPerms(ctx context.Context, id2perms map[int][]string) (res map[int][]string, err error) {
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
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
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	resid2ids = lo.SliceToMap(nodes, func(n *model.Node) (int, int) { return n.Id, n.ResourceId })

	return
}
