package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
	"github.com/veops/oneterm/util"
)

var (
	gatewayPreHooks = []preHook[*model.Gateway]{
		func(ctx *gin.Context, data *model.Gateway) {
			if data.AccountType == model.AUTHMETHOD_PUBLICKEY {
				if data.Phrase == "" {
					_, err := ssh.ParsePrivateKey([]byte(data.Pk))
					if err != nil {
						ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPk, Data: nil})
						return
					}
				} else {
					_, err := ssh.ParsePrivateKeyWithPassphrase([]byte(data.Pk), []byte(data.Phrase))
					if err != nil {
						ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPk, Data: nil})
						return
					}
				}
			}
		},
		func(ctx *gin.Context, data *model.Gateway) {
			data.Password = util.EncryptAES(data.Password)
			data.Pk = util.EncryptAES(data.Pk)
			data.Phrase = util.EncryptAES(data.Phrase)
		},
	}
	gatewayPostHooks = []postHook[*model.Gateway]{
		func(ctx *gin.Context, data []*model.Gateway) {
			post := make([]*model.GatewayCount, 0)
			if err := mysql.DB.
				Model(model.DefaultAsset).
				Select("gateway_id AS id, COUNT(*) AS count").
				Where("gateway_id IN ?", lo.Map(data, func(d *model.Gateway, _ int) int { return d.Id })).
				Group("gateway_id").
				Find(&post).
				Error; err != nil {
				return
			}
			m := lo.SliceToMap(post, func(p *model.GatewayCount) (int, int64) { return p.Id, p.Count })
			for _, d := range data {
				d.AssetCount = m[d.Id]
			}
		},
		func(ctx *gin.Context, data []*model.Gateway) {
			for _, d := range data {
				d.Password = util.DecryptAES(d.Password)
				d.Pk = util.DecryptAES(d.Pk)
				d.Phrase = util.DecryptAES(d.Phrase)
			}
		},
	}
	gatewayDcs = []deleteCheck{
		func(ctx *gin.Context, id int) {
			assetName := ""
			err := mysql.DB.
				Model(model.DefaultAsset).
				Select("name").
				Where("gateway_id = ?", id).
				First(&assetName).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
			code := lo.Ternary(err == nil, http.StatusBadRequest, http.StatusInternalServerError)
			err = lo.Ternary[error](err == nil, &ApiError{Code: ErrHasDepency, Data: map[string]any{"name": assetName}}, err)
			ctx.AbortWithError(code, err)
		},
	}
)

// CreateGateway godoc
//
//	@Tags		gateway
//	@Param		gateway	body		model.Gateway	true	"gateway"
//	@Success	200		{object}	HttpResponse
//	@Router		/gateway [post]
func (c *Controller) CreateGateway(ctx *gin.Context) {
	doCreate(ctx, true, &model.Gateway{}, conf.RESOURCE_GATEWAY, gatewayPreHooks...)
}

// DeleteGateway godoc
//
//	@Tags		gateway
//	@Param		id	path		int	true	"gateway id"
//	@Success	200	{object}	HttpResponse
//	@Router		/gateway/:id [delete]
func (c *Controller) DeleteGateway(ctx *gin.Context) {
	doDelete(ctx, true, &model.Gateway{}, conf.RESOURCE_GATEWAY, gatewayDcs...)
}

// UpdateGateway godoc
//
//	@Tags		gateway
//	@Param		id		path		int				true	"gateway id"
//	@Param		gateway	body		model.Gateway	true	"gateway"
//	@Success	200		{object}	HttpResponse
//	@Router		/gateway/:id [put]
func (c *Controller) UpdateGateway(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Gateway{}, conf.RESOURCE_GATEWAY, gatewayPreHooks...)
}

// GetGateways godoc
//
//	@Tags		gateway
//	@Param		page_index	query		int		true	"gateway id"
//	@Param		page_size	query		int		true	"gateway id"
//	@Param		search		query		string	false	"name or host or account or port"
//	@Param		id			query		int		false	"gateway id"
//	@Param		ids			query		string	false	"gateway ids"
//	@Param		name		query		string	false	"gateway name"
//	@Param		info		query		bool	false	"is info mode"
//	@Param		type		query		int		false	"account type"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Gateway}}
//	@Router		/gateway [get]
func (c *Controller) GetGateways(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	info := cast.ToBool(ctx.Query("info"))

	db := mysql.DB.Model(model.DefaultGateway)
	db = filterEqual(ctx, db, "id", "type")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name", "host", "account", "port")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	if info && !acl.IsAdmin(currentUser) {
		assetIds, err := GetAssetIdsByAuthorization(ctx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		sub := mysql.DB.
			Model(model.DefaultAsset).
			Select("DISTINCT gateway_id").
			Where("asset_id IN ?", assetIds)

		db = db.Where("id IN ?", sub)
	}

	db = db.Order("name")

	doGet(ctx, !info, db, conf.RESOURCE_GATEWAY, gatewayPostHooks...)
}
