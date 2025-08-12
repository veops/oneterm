package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/acl"
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

	// Always exclude sensitive fields (password, pk, phrase) for security
	// These fields require separate MFA-protected API calls to access
	if info {
		db = db.Select("id", "name", "account")
	} else {
		// Exclude sensitive fields but include other metadata
		db = db.Select("id", "name", "account", "account_type", "resource_id",
			"creator_id", "updater_id", "created_at", "updated_at", "deleted_at")
	}

	doGet(ctx, false, db, config.RESOURCE_ACCOUNT, accountPostHooks...)
}

// GetAccountCredentials godoc
//
//	@Tags		account
//	@Summary	Get account credentials with MFA verification
//	@Param		id			path		int		true	"Account ID"
//	@Param		X-MFA-Token	header		string	true	"MFA verification token"
//	@Success	200			{object}	HttpResponse{data=model.Account}
//	@Router		/account/{id}/credentials [post]
func (c *Controller) GetAccountCredentials(ctx *gin.Context) {
	// Get account ID from path parameter
	accountId := cast.ToInt(ctx.Param("id"))
	if accountId == 0 {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{
			Code: myErrors.ErrInvalidArgument,
			Data: map[string]any{"err": "Invalid account ID"},
		})
		return
	}

	// Get MFA token from header
	mfaToken := ctx.GetHeader("X-Mfa-Token")
	if mfaToken == "" {
		ctx.AbortWithError(http.StatusUnauthorized, errors.New("MFA token required in X-MFA-Token header"))
		return
	}

	// Verify MFA token using ACL service
	if !acl.VerifyMFAToken(mfaToken) {
		ctx.AbortWithError(http.StatusUnauthorized, errors.New("MFA token verification failed"))
		return
	}

	account, err := accountService.GetAccountCredentials(ctx, accountId)
	if err != nil {
		if err.Error() == "account not found" {
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{
				Data: map[string]any{"err": "Account not found"},
			})
		} else if err.Error() == "permission denied" {
			ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{
				Code: myErrors.ErrNoPerm,
				Data: map[string]any{"perm": acl.READ},
			})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{
				Code: myErrors.ErrInternal,
				Data: map[string]any{"err": err.Error()},
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: account,
	})
}

// GetAccountCredentials2 godoc
//
//	@Tags		account
//	@Summary	Get account credentials with authorization check only
//	@Param		id		path		int		true	"Account ID"
//	@Success	200		{object}	HttpResponse{data=model.Account}
//	@Router		/account/{id}/credentials2 [get]
func (c *Controller) GetAccountCredentials2(ctx *gin.Context) {
	// Get account ID from path parameter
	accountId := cast.ToInt(ctx.Param("id"))
	if accountId == 0 {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{
			Code: myErrors.ErrInvalidArgument,
			Data: map[string]any{"err": "Invalid account ID"},
		})
		return
	}

	account, err := accountService.GetAccountCredentials(ctx, accountId)
	if err != nil {
		if err.Error() == "account not found" {
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{
				Data: map[string]any{"err": "Account not found"},
			})
		} else if err.Error() == "permission denied" {
			ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{
				Code: myErrors.ErrNoPerm,
				Data: map[string]any{"perm": acl.READ},
			})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{
				Code: myErrors.ErrInternal,
				Data: map[string]any{"err": err.Error()},
			})
		}
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: account,
	})
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
