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
	accountPreHooks = []preHook[*model.Account]{
		func(ctx *gin.Context, data *model.Account) {
			if data.AccountType == model.AUTHMETHOD_PUBLICKEY {
				if data.Phrase == "" {
					_, err := ssh.ParsePrivateKey([]byte(data.Pk))
					if err != nil {
						ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPvk, Data: nil})
						return
					}
				} else {
					_, err := ssh.ParsePrivateKeyWithPassphrase([]byte(data.Pk), []byte(data.Phrase))
					if err != nil {
						ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrWrongPvk, Data: nil})
						return
					}
				}
			}
		},
		func(ctx *gin.Context, data *model.Account) {
			data.Password = util.EncryptAES(data.Password)
			data.Pk = util.EncryptAES(data.Pk)
			data.Phrase = util.EncryptAES(data.Phrase)
		},
	}

	accountPostHooks = []postHook[*model.Account]{
		func(ctx *gin.Context, data []*model.Account) {
			acs := make([]*model.AccountCount, 0)
			if err := mysql.DB.
				Model(&model.Authorization{}).
				Select("account_id AS id, COUNT(*) as count").
				Group("account_id").
				Where("account_id IN ?", lo.Map(data, func(d *model.Account, _ int) int { return d.Id })).
				Find(&acs).
				Error; err != nil {
				return
			}
			m := lo.SliceToMap(acs, func(ac *model.AccountCount) (int, int64) { return ac.Id, ac.Count })
			for _, d := range data {
				d.AssetCount = m[d.Id]
			}
		},
		func(ctx *gin.Context, data []*model.Account) {
			for _, d := range data {
				d.Password = util.DecryptAES(d.Password)
				d.Pk = util.DecryptAES(d.Pk)
				d.Phrase = util.DecryptAES(d.Phrase)
			}
		},
	}
	accountDcs = []deleteCheck{
		func(ctx *gin.Context, id int) {
			assetName := ""
			err := mysql.DB.
				Model(model.DefaultAsset).
				Select("name").
				Where("id = (?)", mysql.DB.Model(&model.Authorization{}).Select("asset_id").Where("account_id = ?", id).Limit(1)).
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

// CreateAccount godoc
//
//	@Tags		account
//	@Param		account	body		model.Account	true	"account"
//	@Success	200		{object}	HttpResponse
//	@Router		/account [post]
func (c *Controller) CreateAccount(ctx *gin.Context) {
	doCreate(ctx, true, &model.Account{}, conf.RESOURCE_ACCOUNT, accountPreHooks...)
}

// DeleteAccount godoc
//
//	@Tags		account
//	@Param		id	path		int	true	"account id"
//	@Success	200	{object}	HttpResponse
//	@Router		/account/:id [delete]
func (c *Controller) DeleteAccount(ctx *gin.Context) {
	doDelete(ctx, true, &model.Account{}, conf.RESOURCE_ACCOUNT, accountDcs...)
}

// UpdateAccount godoc
//
//	@Tags		account
//	@Param		id		path		int				true	"account id"
//	@Param		account	body		model.Account	true	"account"
//	@Success	200		{object}	HttpResponse
//	@Router		/account/:id [put]
func (c *Controller) UpdateAccount(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Account{}, conf.RESOURCE_ACCOUNT, accountPreHooks...)
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

	db := mysql.DB.Model(&model.Account{})
	db = filterEqual(ctx, db, "id", "type")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name", "account")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	if info && !acl.IsAdmin(currentUser) {
		ids, err := GetAccountIdsByAuthorization(ctx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		db = db.Where("id IN ?", ids)
	}

	db = db.Order("name")

	doGet(ctx, !info, db, conf.RESOURCE_ACCOUNT, accountPostHooks...)
}

func GetAccountIdsByAuthorization(ctx *gin.Context) (ids []int, err error) {
	assetIds, err := GetAssetIdsByAuthorization(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	ss := make([]model.Slice[string], 0)
	if err = mysql.DB.Model(model.DefaultAsset).Where("id IN ?", assetIds).Pluck("JSON_KEYS(authorization)", &ss).Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	ids = lo.Uniq(lo.Map(lo.Flatten(ss), func(s string, _ int) int { return cast.ToInt(s) }))
	_, _, accountIds := getIdsByAuthorizationIds(ctx)
	ids = lo.Uniq(append(ids, accountIds...))

	return
}
