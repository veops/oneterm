package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	gsession "github.com/veops/oneterm/session"
)

const (
	kFmtAuthorizationIds = "AuthorizationIds-%d"
	kFmtHasAuthorization = "HasAuthorization-%d-%d-%d"
)

// UpsertAuthorization godoc
//
//	@Tags		authorization
//	@Param		authorization	body		model.Authorization	true	"authorization"
//	@Success	200				{object}	HttpResponse
//	@Router		/authorization [post]
func (c *Controller) UpsertAuthorization(ctx *gin.Context) {
	auth := &model.Authorization{}
	err := ctx.ShouldBindBodyWithJSON(auth)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	if err := mysql.DB.Transaction(func(tx *gorm.DB) error {
		auth := &model.Authorization{}
		if err = tx.Model(auth).
			Where(fmt.Sprintf("node_id %s AND asset_id %s AND account_id %s",
				lo.Ternary(auth.NodeId == nil, "IS NULL", fmt.Sprintf("=%d", auth.NodeId)),
				lo.Ternary(auth.AssetId == nil, "IS NULL", fmt.Sprintf("=%d", auth.AssetId)),
				lo.Ternary(auth.AccountId == nil, "IS NULL", fmt.Sprintf("=%d", auth.AccountId)),
			)).
			FirstOrCreate(auth).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		action := lo.Ternary(auth.Id > 0, model.ACTION_UPDATE, model.ACTION_CREATE)
		return handleAuthorization(ctx, tx, action, nil, auth)
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": auth.GetId(),
		},
	})
}

// DeleteAccount godoc
//
//	@Tags		authorization
//	@Param		id	path		int	true	"authorization id"
//	@Success	200	{object}	HttpResponse
//	@Router		/authorization/:id [delete]
func (c *Controller) DeleteAuthorization(ctx *gin.Context) {
	auth := &model.Authorization{
		Id: cast.ToInt(ctx.Param("id")),
	}
	if err := handleAuthorization(ctx, mysql.DB, model.ACTION_DELETE, nil, auth); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": auth.GetId(),
		},
	})
}

// GetAuthorizations godoc
//
//	@Tags		authorization
//	@Param		page_index	query		int	true	"page_index"
//	@Param		page_size	query		int	true	"page_size"
//	@Param		node_id		query		int	false	"node id"
//	@Param		asset_id	query		int	false	"asset id"
//	@Param		account_id	query		int	false	"account id"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Account}}
//	@Router		/authorization [get]
func (c *Controller) GetAuthorizations(ctx *gin.Context) {
	db := mysql.DB.Model(&model.Authorization{})
	for _, k := range []string{"node_id", "asset_id", "account_id"} {
		q, ok := ctx.GetQuery(k)
		if ok {
			db = db.Where(fmt.Sprintf("%s IN ?", k), lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
		} else {
			db = db.Where(fmt.Sprintf("%s IS NULL", k))
		}
	}

	doGet[*model.Authorization](ctx, false, db, acl.GetResourceTypeName(conf.RESOURCE_AUTHORIZATION))
}

func getAuthsByAsset(t *model.Asset) (data []*model.Authorization, err error) {
	db := mysql.DB.Model(data)
	for accountId := range t.Authorization {
		db = db.Or("asset_id=? AND account_id=? AND node_id=NULL", t.Id, accountId)
	}
	err = db.Find(&data).Error

	return
}

func handleAuthorization(ctx *gin.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) (err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}

	if asset != nil {
		if action == model.ACTION_UPDATE {
			var pres []*model.Authorization
			pres, err = getAuthsByAsset(asset)
			if err != nil {
				return
			}
			for _, p := range pres {
				if _, ok := asset.Authorization[*p.AccountId]; ok {
					auths = append(auths, p)
				} else {
					eg.Go(func() error {
						return acl.DeleteResource(ctx, currentUser.GetUid(), p.ResourceId)
					})
				}
			}
		} else {
			auths = lo.Map(lo.Keys(asset.Authorization), func(id int, _ int) *model.Authorization {
				return &model.Authorization{AssetId: &asset.Id, AccountId: &id, Rids: asset.Authorization[id]}
			})
		}
	}

	for _, a := range lo.Filter(auths, func(item *model.Authorization, _ int) bool { return item != nil }) {
		auth := a
		switch action {
		case model.ACTION_CREATE:
			eg.Go(func() (err error) {
				resourceId := 0
				if resourceId, err = acl.CreateGrantAcl(ctx, currentUser, conf.GetResourceTypeName(conf.RESOURCE_AUTHORIZATION), auth.GetName()); err != nil {
					return
				}
				if err = acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), auth.Rids, resourceId, []string{acl.READ}); err != nil {
					return
				}
				auth.CreatorId = currentUser.GetUid()
				auth.UpdaterId = currentUser.GetUid()
				auth.ResourceId = resourceId
				return tx.Create(auth).Error
			})
		case model.ACTION_DELETE:
			eg.Go(func() (err error) {
				return acl.DeleteResource(ctx, currentUser.GetUid(), auth.ResourceId)
			})
		case model.ACTION_UPDATE:
			eg.Go(func() (err error) {
				pre := &model.Authorization{}
				if err = mysql.DB.First(pre, auth.GetId()).Error; err != nil {
					return
				}
				revokeRids := lo.Without(pre.Rids, auth.Rids...)
				if len(revokeRids) > 0 {
					if err = acl.BatchRevokeRoleResource(ctx, currentUser.GetUid(), revokeRids, auth.ResourceId, []string{acl.READ}); err != nil {
						return
					}
				}
				grantRids := lo.Without(auth.Rids, pre.Rids...)
				if len(grantRids) > 0 {
					err = acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), grantRids, auth.ResourceId, []string{acl.READ})
				}
				return
			})
		}
	}

	err = eg.Wait()

	return
}

func getAutorizationResourceIds(ctx *gin.Context) (resourceIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	k := fmt.Sprintf(kFmtAuthorizationIds, currentUser.GetUid())
	if err = redis.Get(ctx, k, &resourceIds); err == nil {
		return
	}

	var rs []*acl.Resource
	rs, err = acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.RESOURCE_AUTHORIZATION)
	if err != nil {
		return
	}
	resourceIds = lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId })

	redis.SetEx(ctx, k, resourceIds, time.Minute)

	return
}

func hasAuthorization(ctx *gin.Context, sess *gsession.Session) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if sess.ShareId != 0 {
		return true
	}

	k := fmt.Sprintf(kFmtHasAuthorization, currentUser.GetUid(), sess.AccountId, sess.AssetId)
	if err := redis.Get(ctx, k, &ok); err == nil {
		return
	}
	defer func() {
		redis.SetEx(ctx, k, ok, time.Minute)
	}()

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	if sess.Session.Asset == nil {
		if err := mysql.DB.Model(sess.Session.Asset).First(&sess.Session.Asset, sess.AssetId).Error; err != nil {
			return
		}
	}

	authIds, err := getAuthorizationIds(ctx)
	if err != nil {
		return
	}
	if _, ok = lo.Find(authIds, func(item *model.AuthorizationIds) bool {
		return item.NodeId == nil && item.AssetId != nil && *item.AssetId == sess.AssetId && item.AccountId != nil && *item.AccountId == sess.AccountId
	}); ok {
		return
	}
	ctx.Set(kAuthorizationIds, authIds)

	parentNodeIds, assetIds, accountIds := getIdsByAuthorizationIds(ctx)
	tmp, err := handleSelfChild(ctx, parentNodeIds)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}
	parentNodeIds = append(parentNodeIds, tmp...)
	if ok = lo.Contains(parentNodeIds, sess.Session.Asset.ParentId) || lo.Contains(assetIds, sess.AssetId) || lo.Contains(accountIds, sess.AccountId); ok {
		return
	}

	ids, err := getAssetIdsByNodeAccount(ctx, parentNodeIds, accountIds)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}

	return lo.Contains(ids, sess.AssetId)
}
