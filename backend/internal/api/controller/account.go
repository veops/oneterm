package controller

import (
	"errors"
	"net/http"
	"strings"

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
		func(ctx *gin.Context, data *model.Account) {
			var fields map[string]any
			if err := ctx.ShouldBindBodyWithJSON(&fields); err != nil {
				abortPAM(ctx, service.ErrPAMInput)
				return
			}
			for field := range fields {
				switch strings.ToLower(field) {
				case "managed", "authority_kind", "authority_ref", "authority_asset_id", "native_qualifier", "platform", "enabled", "revision":
					abortPAM(ctx, service.ErrPAMInput)
					return
				}
			}
			if err := service.ValidateStoredCredential(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrCredentialInput})
				return
			}
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
	doUpdate(ctx, true, &model.Account{}, config.RESOURCE_ACCOUNT)
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
	if info {
		db = db.Select("id", "name", "account")
	} else {
		// Exclude sensitive fields but include other metadata
		db = db.Select("id", "name", "account", "account_type", "resource_id",
			"managed", "platform", "enabled", "revision",
			"creator_id", "updater_id", "created_at", "updated_at", "deleted_at")
	}

	doGet(ctx, false, db, config.RESOURCE_ACCOUNT, accountPostHooks...)
}

// GetAccountCredentials godoc
//
//	@Tags		account
//	@Summary	Get account credentials with authorization check only
//	@Param		id	path		int	true	"Account ID"
//	@Success	200	{object}	HttpResponse{data=model.Account}
//	@Router		/account/{id}/credentials [post]
func (c *Controller) GetAccountCredentials(ctx *gin.Context) {
	c.writeAccountCredentials(ctx)
}

// GetAccountCredentials2 godoc
//
//	@Tags		account
//	@Summary	Get account credentials with authorization check only
//	@Param		id	path		int	true	"Account ID"
//	@Success	200	{object}	HttpResponse{data=model.Account}
//	@Router		/account/{id}/credentials2 [get]
func (c *Controller) GetAccountCredentials2(ctx *gin.Context) {
	c.writeAccountCredentials(ctx)
}

func (c *Controller) writeAccountCredentials(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
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
		switch {
		case errors.Is(err, service.ErrAccountNotFound):
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{
				Data: map[string]any{"err": "Account not found"},
			})
		case errors.Is(err, service.ErrAccountForbidden):
			ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{
				Code: myErrors.ErrNoPerm,
				Data: map[string]any{"perm": acl.READ},
			})
		default:
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
