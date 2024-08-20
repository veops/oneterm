package controller

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/acl"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/util"
)

var (
	publicKeyPreHooks = []preHook[*model.PublicKey]{
		func(ctx *gin.Context, data *model.PublicKey) {
			if _, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(data.Pk)); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPk, Data: nil})
			} else {
				data.Pk = strings.TrimSpace(strings.TrimSuffix(data.Pk, comment))
			}
			if _, err := net.ParseMAC(data.Mac); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongMac, Data: nil})
			}
		},
		func(ctx *gin.Context, data *model.PublicKey) {
			data.Pk = util.EncryptAES(data.Pk)
		},
		func(ctx *gin.Context, data *model.PublicKey) {
			currentUser, _ := acl.GetSessionFromCtx(ctx)
			data.Uid = currentUser.GetUid()
			data.UserName = currentUser.GetUserName()
		},
	}
	publicKeyPostHooks = []postHook[*model.PublicKey]{
		func(ctx *gin.Context, data []*model.PublicKey) {
			for _, d := range data {
				d.Pk = util.DecryptAES(d.Pk)
			}
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
	doDelete(ctx, false, &model.PublicKey{})
}

// UpdatePublicKey godoc
//
//	@Tags		public_key
//	@Param		id			path		int				true	"publicKey id"
//	@Param		publicKey	body		model.PublicKey	true	"publicKey"
//	@Success	200			{object}	HttpResponse
//	@Router		/public_key/:id [put]
func (c *Controller) UpdatePublicKey(ctx *gin.Context) {
	doUpdate(ctx, false, &model.PublicKey{}, publicKeyPreHooks...)
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
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	db := mysql.DB.Model(&model.PublicKey{})
	db = filterSearch(ctx, db, "name", "mac")
	db = filterEqual(ctx, db, "id")
	db = filterLike(ctx, db, "name")

	db = db.Where("uid = ?", currentUser.Uid)

	doGet[*model.PublicKey](ctx, false, db, "", publicKeyPostHooks...)
}

// Auth godoc
//
//	@Tags		public_key
//	@Param		req	body		model.ReqAuth	false	"method 1password 2publickey"
//	@Success	200	{object}	HttpResponse{}
//	@Router		/public_key/auth [post]
func (c *Controller) Auth(ctx *gin.Context) {
	data := &model.ReqAuth{}
	if err := ctx.BindJSON(data); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	switch data.Method {
	case model.AUTHMETHOD_PASSWORD:
		cookie, err := acl.LoginByPassword(ctx, data.UserName, data.Password)
		if err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
			return
		}
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(map[string]any{"cookie": cookie}))
	case model.AUTHMETHOD_PUBLICKEY:
		pk := &model.PublicKey{}
		if err := mysql.DB.
			Where("username = ? AND pk = ?", data.UserName, util.EncryptAES(data.Pk)).
			First(pk).Error; err != nil {
			ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
			return
		}
		cookie, err := acl.LoginByPublicKey(ctx, data.UserName)
		if err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
			return
		}
		ctx.JSON(http.StatusOK, NewHttpResponseWithData(map[string]any{"cookie": cookie}))
	default:
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": "invalid auth method"}})
		return
	}
}
