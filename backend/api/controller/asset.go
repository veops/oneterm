package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/schedule"
)

const (
	kFmtAssetIds      = "assetIds-%d"
	kAuthorizationIds = "authorizationIds"
	kParentNodeIds    = "parentNodeIds"
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
	doDelete(ctx, true, &model.Asset{})
}

// UpdateAsset godoc
//
//	@Tags		asset
//	@Param		id		path		int			true	"asset id"
//	@Param		asset	body		model.Asset	true	"asset"
//	@Success	200		{object}	HttpResponse
//	@Router		/asset/:id [put]
func (c *Controller) UpdateAsset(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Asset{})
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

	db := mysql.DB.Model(&model.Asset{})
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
			return
		}
		db = db.Where("id IN ?", ids)
	}

	db = db.Order("name")

	doGet(ctx, !info, db, acl.GetResourceTypeName(conf.RESOURCE_AUTHORIZATION), assetPostHooks...)
}

func assetPostHookCount(ctx *gin.Context, data []*model.Asset) {
	nodes := make([]*model.NodeIdPidName, 0)
	if err := mysql.DB.
		Model(nodes).
		Find(&nodes).
		Error; err != nil {
		logger.L().Error("asset posthookfailed", zap.Error(err))
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
	authorizationIds, _ := ctx.Value("authorizationIds").([]*model.AuthorizationIds)
	parentNodeIds, _, accountIds := getIdsByAuthorizationIds(ctx)
	for _, a := range data {
		if lo.Contains(parentNodeIds, a.Id) {
			continue
		}
		ids := lo.Uniq(
			lo.Map(lo.Filter(authorizationIds, func(item *model.AuthorizationIds, _ int) bool {
				return item.AssetId != nil && *item.AssetId == a.Id && item.AccountId != nil
			}),
				func(item *model.AuthorizationIds, _ int) int { return *item.AccountId }))

		for k := range a.Authorization {
			if !lo.Contains(ids, k) && !lo.Contains(accountIds, k) {
				delete(a.Authorization, k)
			}
		}
	}
}

func handleParentId(ctx context.Context, parentId int) (pids []int, err error) {
	nodes := make([]*model.NodeIdPid, 0)
	if err = redis.Get(ctx, kFmtAllNodes, &nodes); err != nil {
		if err = mysql.DB.Model(&model.Node{}).Find(&nodes).Error; err != nil {
			return
		}
		redis.SetEx(ctx, kFmtAllNodes, nodes, time.Hour)
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
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	authIds, err := getAuthorizationIds(ctx)
	if err != nil {
		return
	}
	ctx.Set(kAuthorizationIds, authIds)

	k := fmt.Sprintf(kFmtAssetIds, currentUser.GetUid())
	if err = redis.Get(ctx, k, &ids); err == nil {
		return
	}

	parentNodeIds, ids, accountIds := getIdsByAuthorizationIds(ctx)

	tmp, err := handleSelfChild(ctx, parentNodeIds)
	if err != nil {
		return
	}
	parentNodeIds = append(parentNodeIds, tmp...)
	ctx.Set(kParentNodeIds, parentNodeIds)
	ctx.Set(kAccountIds, accountIds)
	tmp, err = getAssetIdsByNodeAccount(ctx, parentNodeIds, accountIds)
	if err != nil {
		return
	}
	ids = lo.Uniq(append(ids, tmp...))

	redis.SetEx(ctx, k, ids, time.Minute)

	return
}

func getIdsByAuthorizationIds(ctx context.Context) (parentNodeIds, assetIds, accountIds []int) {
	authIds, _ := ctx.Value(kAuthorizationIds).([]*model.AuthorizationIds)

	for _, a := range authIds {
		if a.NodeId != nil && a.AssetId == nil && a.AccountId == nil {
			parentNodeIds = append(parentNodeIds, *a.NodeId)
		}
		if a.AssetId != nil && a.NodeId == nil && a.AccountId == nil {
			assetIds = append(assetIds, *a.AssetId)
		}
		if a.AccountId != nil && a.AssetId == nil && a.NodeId == nil {
			accountIds = append(accountIds, *a.AccountId)
		}
	}
	return
}

func getAuthorizationIds(ctx *gin.Context) (authIds []*model.AuthorizationIds, err error) {
	resourceIds, err := getAutorizationResourceIds(ctx)
	if err != nil {
		handleRemoteErr(ctx, err)
		return
	}

	err = mysql.DB.Model(authIds).Where("resource_id IN ?", resourceIds).Find(&authIds).Error
	return
}

func getAssetIdsByNodeAccount(ctx context.Context, parentNodeIds, accountIds []int) (assetIds []int, err error) {
	err = mysql.DB.Model(&model.Asset{}).Where("parent_id IN?", parentNodeIds).Or("JSON_KEYS(authorization) IN ?", accountIds).Pluck("id", &assetIds).Error
	return
}
