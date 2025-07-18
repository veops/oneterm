package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

var (
	accountService = service.NewAccountService()

	accountPreHooks = []preHook[*model.Account]{
		// Validate public key
		func(ctx *gin.Context, data *model.Account) {
			if err := accountService.ValidatePublicKey(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrWrongPvk, Data: nil})
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
			err = lo.Ternary[error](err == nil, &myErrors.ApiError{Code: myErrors.ErrHasDepency, Data: map[string]any{"name": assetName}}, err)
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
	info := cast.ToBool(ctx.Query("info"))

	// Build query with integrated V2 authorization filter
	db, err := accountService.BuildQueryWithAuthorization(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply info mode settings
	if info {
		db = db.Select("id", "name", "account")
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
