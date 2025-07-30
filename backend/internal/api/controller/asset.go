package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/samber/lo"
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
		// Filter web_config based on write permissions
		func(ctx *gin.Context, data []*model.Asset) {
			for _, asset := range data {
				if asset.Permissions == nil || !lo.Contains(asset.Permissions, acl.WRITE) {
					asset.WebConfig = nil
				}
			}
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
	info := cast.ToBool(ctx.Query("info"))

	// Build query with integrated V2 authorization filter
	db, err := assetService.BuildQueryWithAuthorization(ctx)
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
		db = db.Select("id", "parent_id", "name", "ip", "protocols", "connectable", "authorization", "resource_id", "access_time_control", "asset_command_control", "web_config")
	}

	doGet(ctx, false, db, config.RESOURCE_ASSET, assetPostHooks...)
}

// GetAssetIdsByAuthorization gets asset IDs by authorization
func GetAssetIdsByAuthorization(ctx *gin.Context) ([]int, error) {
	// Use V2 authorization system for asset filtering
	authV2Service := service.NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	return assetIds, err
}

// AssetPermissionResult represents simplified permission result without permissions field
type AssetPermissionResult struct {
	Allowed      bool                   `json:"allowed"`
	Reason       string                 `json:"reason"`
	RuleId       int                    `json:"rule_id"`
	RuleName     string                 `json:"rule_name"`
	Restrictions map[string]interface{} `json:"restrictions"`
}

// AssetPermissionBatchResult represents batch permission results without permissions field
type AssetPermissionBatchResult struct {
	Results map[model.AuthAction]*AssetPermissionResult `json:"results"`
}

// AssetPermissionMultiAccountResult represents permission results for multiple accounts
type AssetPermissionMultiAccountResult struct {
	Results map[int]*AssetPermissionBatchResult `json:"results"` // accountId -> batch results
}

// GetAssetPermissions godoc
//
//	@Tags		asset
//	@Param		id			path		int		true	"asset id"
//	@Param		account_ids	query		string	false	"account ids (comma separated, e.g. 123,456,789)"
//	@Success	200			{object}	HttpResponse{data=AssetPermissionMultiAccountResult}
//	@Router		/asset/:id/permissions [get]
func (c *Controller) GetAssetPermissions(ctx *gin.Context) {
	assetId := cast.ToInt(ctx.Param("id"))
	accountIdsStr := ctx.Query("account_ids")

	if assetId <= 0 {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": "invalid asset id"}})
		return
	}

	// Parse account_ids parameter
	var accountIds []int
	if accountIdsStr != "" {
		// Split by comma and convert to integers
		idStrs := strings.Split(accountIdsStr, ",")
		for _, idStr := range idStrs {
			idStr = strings.TrimSpace(idStr)
			if idStr != "" {
				if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
					accountIds = append(accountIds, id)
				}
			}
		}
	}

	// Remove duplicates
	accountIds = lo.Uniq(accountIds)

	// Use V2 authorization service to get permissions
	authV2Service := service.NewAuthorizationV2Service()

	// If no account IDs provided, return empty result
	if len(accountIds) == 0 {
		ctx.JSON(http.StatusOK, HttpResponse{
			Data: &AssetPermissionBatchResult{
				Results: make(map[model.AuthAction]*AssetPermissionResult),
			},
		})
		return
	}

	multiResult := &AssetPermissionMultiAccountResult{
		Results: make(map[int]*AssetPermissionBatchResult),
	}

	for _, accId := range accountIds {
		result, err := authV2Service.GetAssetPermissions(ctx, assetId, accId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}

		// Convert to simplified result without permissions field
		simplifiedResult := &AssetPermissionBatchResult{
			Results: make(map[model.AuthAction]*AssetPermissionResult),
		}

		for action, authResult := range result.Results {
			simplifiedResult.Results[action] = &AssetPermissionResult{
				Allowed:      authResult.Allowed,
				Reason:       authResult.Reason,
				RuleId:       authResult.RuleId,
				RuleName:     authResult.RuleName,
				Restrictions: authResult.Restrictions,
			}
		}

		multiResult.Results[accId] = simplifiedResult
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: multiResult,
	})
}
