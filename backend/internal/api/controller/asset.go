package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	kFmtAssetIds      = "assetIds-%d"
	kAuthorizationIds = "authorizationIds"
	kNodeIds          = "nodeIds"
	kAccountIds       = "accountIds"
)

var (
	assetService = service.NewAssetService()

	assetPreHooks = []preHook[*model.Asset]{
		// Preprocess asset data
		func(ctx *gin.Context, data *model.Asset) {
			assetService.PreprocessAssetData(data)
		},
	}
	assetPostHooks = []postHook[*model.Asset]{
		// Attach node chain
		func(ctx *gin.Context, data []*model.Asset) {
			if err := assetService.AttachNodeChain(ctx, data); err != nil {
				logger.L().Error("attach node chain failed", zap.Error(err))
				return
			}
		},
		// Apply authorization filters
		func(ctx *gin.Context, data []*model.Asset) {
			currentUser, _ := acl.GetSessionFromCtx(ctx)
			if acl.IsAdmin(currentUser) {
				return
			}

			authorizationIds, _ := ctx.Value(kAuthorizationIds).([]*model.AuthorizationIds)
			nodeIds, _ := ctx.Value(kNodeIds).([]int)
			accountIds, _ := ctx.Value(kAccountIds).([]int)

			assetService.ApplyAuthorizationFilters(ctx, data, authorizationIds, nodeIds, accountIds)
		},
	}
)

// CreateAsset godoc
//
//	@Tags		asset
//	@Param		asset	body		model.Asset	true	"asset"
//	@Success	200		{object}	HttpResponse
//	@Router		/asset [post]
func (c *Controller) CreateAsset(ctx *gin.Context) {
	asset := &model.Asset{}
	doCreate(ctx, true, asset, config.RESOURCE_ASSET, assetPreHooks...)

	assetService.UpdateConnectables(asset.Id)
}

// DeleteAsset godoc
//
//	@Tags		asset
//	@Param		id	path		int	true	"asset id"
//	@Success	200	{object}	HttpResponse
//	@Router		/asset/:id [delete]
func (c *Controller) DeleteAsset(ctx *gin.Context) {
	doDelete(ctx, true, &model.Asset{}, config.RESOURCE_ASSET)
}

// UpdateAsset godoc
//
//	@Tags		asset
//	@Param		id		path		int			true	"asset id"
//	@Param		asset	body		model.Asset	true	"asset"
//	@Success	200		{object}	HttpResponse
//	@Router		/asset/:id [put]
func (c *Controller) UpdateAsset(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Asset{}, config.RESOURCE_ASSET, assetPreHooks...)
	assetService.UpdateConnectables(cast.ToInt(ctx.Param("id")))
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

	// Build base query using service layer
	db, err := assetService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply parent_id filter if needed
	if q, ok := ctx.GetQuery("parent_id"); ok {
		db, err = assetService.FilterByParentId(db, cast.ToInt(q))
		if err != nil {
			logger.L().Error("parent id filtering failed", zap.Error(err))
			ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}
	}

	// Apply info mode settings
	if info {
		db = db.Select("id", "parent_id", "name", "ip", "protocols", "connectable", "authorization")

		// Apply authorization filter if needed
		if !acl.IsAdmin(currentUser) {
			ids, err := GetAssetIdsByAuthorization(ctx)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
				return
			}
			db = db.Where("id IN ?", ids)
		}
	}

	doGet(ctx, !info, db, config.RESOURCE_ASSET, assetPostHooks...)
}

// GetAssetIdsByAuthorization gets asset IDs by authorization
func GetAssetIdsByAuthorization(ctx *gin.Context) ([]int, error) {
	_, assetIds, _, err := assetService.GetAssetIdsByAuthorization(ctx)
	return assetIds, err
}
