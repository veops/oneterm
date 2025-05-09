package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/errors"
)

var (
	publicKeyService = service.NewPublicKeyService()

	publicKeyPreHooks = []preHook[*model.PublicKey]{
		func(ctx *gin.Context, data *model.PublicKey) {
			if err := publicKeyService.ValidatePublicKey(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrWrongPk, Data: nil})
			}
		},
		func(ctx *gin.Context, data *model.PublicKey) {
			publicKeyService.EncryptPublicKey(data)
		},
		func(ctx *gin.Context, data *model.PublicKey) {
			publicKeyService.SetUserInfo(ctx, data)
		},
	}

	publicKeyPostHooks = []postHook[*model.PublicKey]{
		func(ctx *gin.Context, data []*model.PublicKey) {
			publicKeyService.DecryptPublicKeys(data)
		},
	}
)

// CreatePublicKey godoc
//
//	@Tags		public_key
//	@Param		publicKey	body		model.PublicKey	true	"publicKey"
//	@Success	200			{object}	HttpResponse
//	@Router		/public_key [post]
func (c *Controller) CreatePublicKey(ctx *gin.Context) {
	doCreate(ctx, false, &model.PublicKey{}, "", publicKeyPreHooks...)
}

// DeletePublicKey godoc
//
//	@Tags		public_key
//	@Param		id	path		int	true	"publicKey id"
//	@Success	200	{object}	HttpResponse
//	@Router		/public_key/:id [delete]
func (c *Controller) DeletePublicKey(ctx *gin.Context) {
	doDelete(ctx, false, &model.PublicKey{}, "")
}

// UpdatePublicKey godoc
//
//	@Tags		public_key
//	@Param		id			path		int				true	"publicKey id"
//	@Param		publicKey	body		model.PublicKey	true	"publicKey"
//	@Success	200			{object}	HttpResponse
//	@Router		/public_key/:id [put]
func (c *Controller) UpdatePublicKey(ctx *gin.Context) {
	doUpdate(ctx, false, &model.PublicKey{}, "", publicKeyPreHooks...)
}

// GetPublicKeys godoc
//
//	@Tags		public_key
//	@Param		page_index	query		int		true	"publicKey id"
//	@Param		page_size	query		int		true	"publicKey id"
//	@Param		search		query		string	false	"name or mac"
//	@Param		id			query		int		false	"publicKey id"
//	@Param		name		query		string	false	"publicKey name"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.PublicKey}}
//	@Router		/public_key [get]
func (c *Controller) GetPublicKeys(ctx *gin.Context) {
	db := publicKeyService.BuildQuery(ctx)
	doGet(ctx, false, db, "", publicKeyPostHooks...)
}
