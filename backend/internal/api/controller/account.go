package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
)

var (
	accountService = service.NewAccountService()

	accountPreHooks = []preHook[*model.Account]{
		// Validate public key
		func(ctx *gin.Context, data *model.Account) {
			if err := accountService.ValidatePublicKey(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPvk, Data: nil})
				return
			}
		},
		// Encrypt sensitive data
		func(ctx *gin.Context, data *model.Account) {
			accountService.EncryptSensitiveData(data)
		},
	}

	accountPostHooks = []postHook[*model.Account]{
		// Attach asset count
		func(ctx *gin.Context, data []*model.Account) {
			if err := accountService.AttachAssetCount(ctx, data); err != nil {
				return
			}
		},
		// Decrypt sensitive data
		func(ctx *gin.Context, data []*model.Account) {
			accountService.DecryptSensitiveData(data)
		},
	}

	accountDcs = []deleteCheck{
		// Check dependencies
		func(ctx *gin.Context, id int) {
			assetName, err := accountService.CheckAssetDependencies(ctx, id)
			if err == nil && assetName == "" {
				return
			}
			code := lo.Ternary(err == nil, http.StatusBadRequest, http.StatusInternalServerError)
			err = lo.Ternary[error](err == nil, &ApiError{Code: ErrHasDepency, Data: map[string]any{"name": assetName}}, err)
			ctx.AbortWithError(code, err)
		},
	}
)

// CreateAccount godoc
//
//	@Tags		account
//	@Param		account	body		model.Account	true	"account"
//	@Success	200		{object}	HttpResponse
//	@Router		/account [post]
func (c *Controller) CreateAccount(ctx *gin.Context) {
	doCreate(ctx, true, &model.Account{}, config.RESOURCE_ACCOUNT, accountPreHooks...)
}

// DeleteAccount godoc
//
//	@Tags		account
//	@Param		id	path		int	true	"account id"
//	@Success	200	{object}	HttpResponse
//	@Router		/account/:id [delete]
func (c *Controller) DeleteAccount(ctx *gin.Context) {
	doDelete(ctx, true, &model.Account{}, config.RESOURCE_ACCOUNT, accountDcs...)
}

// UpdateAccount godoc
//
//	@Tags		account
//	@Param		id		path		int				true	"account id"
//	@Param		account	body		model.Account	true	"account"
//	@Success	200		{object}	HttpResponse
//	@Router		/account/:id [put]
func (c *Controller) UpdateAccount(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Account{}, config.RESOURCE_ACCOUNT, accountPreHooks...)
}

// GetAccounts godoc
//
//	@Tags		account
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		search		query		string	false	"name or account"
//	@Param		id			query		int		false	"account id"
//	@Param		ids			query		string	false	"account ids"
//	@Param		name		query		string	false	"account name"
//	@Param		info		query		bool	false	"is info mode"
//	@Param		type		query		int		false	"account type"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Account}}
//	@Router		/account [get]
func (c *Controller) GetAccounts(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	info := cast.ToBool(ctx.Query("info"))

	// Build base query using service layer
	db := accountService.BuildQuery(ctx)

	// Apply select fields for info mode
	if info {
		db = db.Select("id", "name", "account")

		// Apply authorization filter if needed
		if !acl.IsAdmin(currentUser) {
			assetIds, err := GetAssetIdsByAuthorization(ctx)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
				return
			}

			// Filter accounts by asset IDs
			db = accountService.FilterByAssetIds(db, assetIds)
		}
	}

	doGet(ctx, !info, db, config.RESOURCE_ACCOUNT, accountPostHooks...)
}

// GetAccountIdsByAuthorization gets account IDs by authorization
func GetAccountIdsByAuthorization(ctx *gin.Context) ([]int, error) {
	assetIds, err := GetAssetIdsByAuthorization(ctx)
	if err != nil {
		return nil, err
	}

	_, _, authorizationIds := getIdsByAuthorizationIds(ctx)

	return accountService.GetAccountIdsByAuthorization(ctx, assetIds, authorizationIds)
}
