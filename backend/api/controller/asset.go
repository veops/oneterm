package controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/schedule"
	"github.com/veops/oneterm/util"
)

const (
	kFmtAssetIds      = "assetIds-%d"
	kAuthorizationIds = "authorizationIds"
	kNodeIds          = "nodeIds"
	kAccountIds       = "accountIds"
)

var (
	assetPreHooks = []preHook[*model.Asset]{
		func(ctx *gin.Context, data *model.Asset) {
			data.Ip = strings.TrimSpace(data.Ip)
			data.Protocols = lo.Map(data.Protocols, func(s string, _ int) string { return strings.TrimSpace(s) })
			if data.Authorization == nil {
				data.Authorization = make(model.Map[int, model.Slice[int]])
			}
		},
	}
	assetPostHooks = []postHook[*model.Asset]{assetPostHookCount, assetPostHookAuth}
)

// CreateAsset godoc
//
//	@Tags		asset
//	@Param		asset	body		model.Asset	true	"asset"
//	@Success	200		{object}	HttpResponse
//	@Router		/asset [post]
func (c *Controller) CreateAsset(ctx *gin.Context) {
	asset := &model.Asset{}
	doCreate(ctx, true, asset, conf.RESOURCE_ASSET, assetPreHooks...)

	schedule.UpdateConnectables(asset.Id)
}

// DeleteAsset godoc
//
//	@Tags		asset
//	@Param		id	path		int	true	"asset id"
//	@Success	200	{object}	HttpResponse
//	@Router		/asset/:id [delete]
func (c *Controller) DeleteAsset(ctx *gin.Context) {
	doDelete(ctx, true, &model.Asset{}, conf.RESOURCE_ASSET)
}

// UpdateAsset godoc
//
//	@Tags		asset
//	@Param		id		path		int			true	"asset id"
//	@Param		asset	body		model.Asset	true	"asset"
//	@Success	200		{object}	HttpResponse
//	@Router		/asset/:id [put]
func (c *Controller) UpdateAsset(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Asset{}, conf.RESOURCE_ASSET)
	schedule.UpdateConnectables(cast.ToInt(ctx.Param("id")))
}

// GetAssets godoc
//
//	@Tags		asset
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		search		query		string	false	"name or ip"
//	@Param		id			query		int		false	"asset id"
//	@Param		ids			query		string	false	"asset ids"
//	@Param		parent_id	query		int		false	"asset's parent id"
//	@Param		name		query		string	false	"asset name"
//	@Param		ip			query		string	false	"asset ip"
//	@Param		info		query		bool	false	"is info mode"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Asset}}
//	@Router		/asset [get]
func (c *Controller) GetAssets(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	info := cast.ToBool(ctx.Query("info"))

	db := mysql.DB.Model(model.DefaultAsset)
	db = filterEqual(ctx, db, "id")
	db = filterLike(ctx, db, "name", "ip")
	db = filterSearch(ctx, db, "name", "ip")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}
	if q, ok := ctx.GetQuery("parent_id"); ok {
		parentIds, err := handleParentId(ctx, cast.ToInt(q))
		if err != nil {
			logger.L().Error("parent id found failed", zap.Error(err))
			return
		}
		db = db.Where("parent_id IN ?", parentIds)
	}

	if info && !acl.IsAdmin(currentUser) {
		ids, err := GetAssetIdsByAuthorization(ctx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		db = db.Where("id IN ?", ids)
	}

	db = db.Order("name")

	doGet(ctx, !info, db, conf.RESOURCE_ASSET, assetPostHooks...)
}

func assetPostHookCount(ctx *gin.Context, data []*model.Asset) {
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	g := make(map[int][]model.Pair[int, string])
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], model.Pair[int, string]{First: n.Id, Second: n.Name})
	}
	m := make(map[int]string)
	var dfs func(int, string)
	dfs = func(x int, s string) {
		m[x] = s
		for _, node := range g[x] {
			dfs(node.First, fmt.Sprintf("%s/%s", s, node.Second))
		}
	}
	dfs(0, "")

	for _, d := range data {
		d.NodeChain = m[d.ParentId]
	}
}

func assetPostHookAuth(ctx *gin.Context, data []*model.Asset) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if acl.IsAdmin(currentUser) {
		return
	}
	info := cast.ToBool(ctx.Query("info"))
	noInfoIds := make([]int, 0)
	if !info {
		t := mysql.DB.Model(model.DefaultAsset)
		assetResIds, _ := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), conf.RESOURCE_ASSET)
		t, _ = handleAssetIds(ctx, t, assetResIds)
		t.Pluck("id", &noInfoIds)
	}

	authorizationIds, _ := ctx.Value(kAuthorizationIds).([]*model.AuthorizationIds)
	nodeIds, _, accountIds := getIdsByAuthorizationIds(ctx)
	nodeIds, _ = handleSelfChild(ctx, nodeIds...)

	for _, a := range data {
		if lo.Contains(nodeIds, a.ParentId) || lo.Contains(noInfoIds, a.Id) {
			continue
		}
		if lo.ContainsBy(authorizationIds, func(item *model.AuthorizationIds) bool {
			return item.AssetId == a.Id && item.NodeId == 0 && item.AccountId == 0
		}) {
			continue
		}
		ids := lo.Map(lo.Filter(authorizationIds, func(item *model.AuthorizationIds, _ int) bool {
			return item.AssetId == a.Id && item.AccountId != 0 && item.NodeId == 0
		}),
			func(item *model.AuthorizationIds, _ int) int { return item.AccountId })

		for k := range a.Authorization {
			if !lo.Contains(ids, k) && !lo.Contains(accountIds, k) {
				delete(a.Authorization, k)
			}
		}
	}
}

func handleParentId(ctx context.Context, parentId int) (pids []int, err error) {
	nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}
	var dfs func(int)
	dfs = func(x int) {
		pids = append(pids, x)
		for _, y := range g[x] {
			dfs(y)
		}
	}
	dfs(parentId)

	return
}

func GetAssetIdsByAuthorization(ctx *gin.Context) (ids []int, err error) {
	authIds, err := getAuthorizationIds(ctx)
	if err != nil {
		return
	}
	ctx.Set(kAuthorizationIds, authIds)

	nodeIds, ids, accountIds := getIdsByAuthorizationIds(ctx)

	tmp, err := handleSelfChild(ctx, nodeIds...)
	if err != nil {
		return
	}
	nodeIds = append(nodeIds, tmp...)
	ctx.Set(kNodeIds, nodeIds)
	ctx.Set(kAccountIds, accountIds)
	tmp, err = getAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
	if err != nil {
		return
	}
	ids = lo.Uniq(append(ids, tmp...))

	return
}

func getIdsByAuthorizationIds(ctx *gin.Context) (nodeIds, assetIds, accountIds []int) {
	authIds, _ := ctx.Value(kAuthorizationIds).([]*model.AuthorizationIds)
	info := cast.ToBool(ctx.Query("info"))
	for _, a := range authIds {
		if a.NodeId != 0 && a.AssetId == 0 && a.AccountId == 0 {
			nodeIds = append(nodeIds, a.NodeId)
		}
		if a.AssetId != 0 && a.NodeId == 0 && (info || a.AccountId == 0) {
			assetIds = append(assetIds, a.AssetId)
		}
		if a.AccountId != 0 && a.AssetId == 0 && (info || a.NodeId == 0) {
			accountIds = append(accountIds, a.AccountId)
		}
	}
	return
}

func getAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) (assetIds []int, err error) {
	assets, err := util.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return
	}
	assets = lo.Filter(assets, func(a *model.Asset, _ int) bool {
		return lo.Contains(nodeIds, a.ParentId) || len(lo.Intersect(lo.Keys(a.Authorization), accountIds)) > 0
	})
	assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })

	return
}
