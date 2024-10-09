package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	gsession "github.com/veops/oneterm/session"
	"github.com/veops/oneterm/util"
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
		t := &model.Authorization{}
		if err = tx.Model(t).
			Where("node_id=? AND asset_id=? AND account_id=?", auth.NodeId, auth.AssetId, auth.AccountId).
			First(t).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			err = nil
		} else {
			auth.Id = t.Id
			auth.ResourceId = t.ResourceId
		}
		if !hasPermAuthorization(ctx, auth, acl.GRANT) {
			err = &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}}
			ctx.AbortWithError(http.StatusForbidden, err)
			return err
		}
		action := lo.Ternary(auth.Id > 0, model.ACTION_UPDATE, model.ACTION_CREATE)
		return handleAuthorization(ctx, tx, action, nil, auth)
	}); err != nil {
		if ctx.IsAborted() {
			return
		}
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

	if err := mysql.DB.Model(auth).Where("id=?", auth.Id).First(auth); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	if !hasPermAuthorization(ctx, auth, acl.GRANT) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
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
	auth := &model.Authorization{
		AssetId:   cast.ToInt(ctx.Query("asset_id")),
		AccountId: cast.ToInt(ctx.Query("account_id")),
		NodeId:    cast.ToInt(ctx.Query("node_id")),
	}
	db := mysql.DB.Model(auth)
	for _, k := range []string{"node_id", "asset_id", "account_id"} {
		q, _ := ctx.GetQuery(k)
		db = db.Where(fmt.Sprintf("%s=?", k), cast.ToInt(q))
	}
	t := db.Session(&gorm.Session{})

	if err := t.First(&auth).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	if !hasPermAuthorization(ctx, auth, acl.GRANT) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
	}

	doGet[*model.Authorization](ctx, false, db, conf.RESOURCE_AUTHORIZATION)
}

func getNodeAssetAccoutIdsByAction(ctx context.Context, action string) (nodeIds, assetIds, accountIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}
	ch := make(chan bool)

	eg.Go(func() (err error) {
		defer close(ch)
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), conf.RESOURCE_NODE)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		nodes, err := util.GetAllFromCacheDb(ctx, model.DefaultNode)
		if err != nil {
			return
		}
		nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(resIds, n.ResourceId) })
		nodeIds = lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })
		nodeIds, err = handleSelfChild(ctx, nodeIds...)
		return
	})

	eg.Go(func() (err error) {
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), conf.RESOURCE_ASSET)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		<-ch
		assets, err := util.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return
		}
		assets = lo.Filter(assets, func(a *model.Asset, _ int) bool {
			return lo.Contains(resIds, a.ResourceId) || lo.Contains(nodeIds, a.ParentId)
		})
		assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })
		return
	})

	eg.Go(func() (err error) {
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), conf.RESOURCE_ACCOUNT)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		accounts, err := util.GetAllFromCacheDb(ctx, model.DefaultAccount)
		if err != nil {
			return
		}
		accounts = lo.Filter(accounts, func(a *model.Account, _ int) bool { return lo.Contains(resIds, a.ResourceId) })
		accountIds = lo.Map(accounts, func(a *model.Account, _ int) int { return a.Id })
		return
	})

	err = eg.Wait()

	return
}

func hasPermAuthorization(ctx context.Context, auth *model.Authorization, action string) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	if auth == nil {
		auth = &model.Authorization{}
	}

	nodeIds, assetIds, accountIds, err := getNodeAssetAccoutIdsByAction(ctx, action)
	if err != nil {
		return
	}

	if auth.NodeId != 0 && auth.AssetId == 0 && auth.AccountId == 0 {
		ok = lo.Contains(nodeIds, auth.NodeId)
	} else if auth.AssetId != 0 && auth.NodeId == 0 && auth.AccountId == 0 {
		ok = lo.Contains(assetIds, auth.AssetId)
	} else if auth.AccountId != 0 && auth.AssetId == 0 && auth.NodeId == 0 {
		ok = lo.Contains(accountIds, auth.AccountId)
	}

	return
}

func getAuthsByAsset(t *model.Asset) (data []*model.Authorization, err error) {
	err = mysql.DB.Model(data).Where("asset_id=? AND account_id IN ? AND node_id=0", t.Id, lo.Without(lo.Keys(t.Authorization), 0)).Find(&data).Error
	return
}

func handleAuthorization(ctx *gin.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) (err error) {
	defer util.DeleteAllFromCacheDb(ctx, model.DefaultAuthorization)

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}

	if asset != nil && asset.Id > 0 {
		var pres []*model.Authorization
		pres, err = getAuthsByAsset(asset)
		if err != nil {
			return
		}
		switch action {
		case model.ACTION_CREATE:
			auths = lo.Map(lo.Keys(asset.Authorization), func(id int, _ int) *model.Authorization {
				return &model.Authorization{AssetId: asset.Id, AccountId: id, Rids: asset.Authorization[id]}
			})
		case model.ACTION_DELETE:
			auths = pres
		case model.ACTION_UPDATE:
			for _, pre := range pres {
				p := pre
				if v, ok := asset.Authorization[p.AccountId]; ok {
					p.Rids = v
					auths = append(auths, p)
				} else {
					eg.Go(func() (err error) {
						if err = acl.DeleteResource(ctx, currentUser.GetUid(), p.ResourceId); err != nil {
							return
						}
						if err = mysql.DB.Model(p).Where("id=?", p.Id).Delete(p).Error; err != nil {
							return
						}
						return
					})
				}
			}
			preAccountsIds := lo.Map(pres, func(p *model.Authorization, _ int) int { return p.AccountId })
			for k, v := range asset.Authorization {
				if !lo.Contains(preAccountsIds, k) {
					auths = append(auths, &model.Authorization{AssetId: asset.Id, AccountId: k, Rids: v})
				}
			}
		}
	}

	for _, a := range lo.Filter(auths, func(item *model.Authorization, _ int) bool { return item != nil }) {
		auth := a
		switch action {
		case model.ACTION_CREATE:
			eg.Go(func() (err error) {
				resourceId := 0
				if resourceId, err = acl.CreateAcl(ctx, currentUser, conf.RESOURCE_AUTHORIZATION, auth.GetName()); err != nil {
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
				if err = mysql.DB.Where("id=?", auth.GetId()).First(pre).Error; err != nil {
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						return
					}
					resourceId := 0
					if resourceId, err = acl.CreateAcl(ctx, currentUser, conf.RESOURCE_AUTHORIZATION, auth.GetName()); err != nil {
						return
					}
					auth.ResourceId = resourceId
					if err = tx.Create(auth).Error; err != nil {
						return
					}
				}
				revokeRids := lo.Without(pre.Rids, auth.Rids...)
				if len(revokeRids) > 0 {
					if err = acl.BatchRevokeRoleResource(ctx, currentUser.GetUid(), revokeRids, auth.ResourceId, []string{acl.READ}); err != nil {
						return
					}
				}
				grantRids := lo.Without(auth.Rids, pre.Rids...)
				if len(grantRids) > 0 {
					if err = acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), grantRids, auth.ResourceId, []string{acl.READ}); err != nil {
						return
					}
				}
				return tx.Model(auth).Update("rids", auth.Rids).Error
			})
		}
	}

	err = eg.Wait()

	return
}

func getAuthorizations(ctx *gin.Context) (res []*acl.Resource, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	res, err = acl.GetRoleResources(ctx, currentUser.GetRid(), conf.RESOURCE_AUTHORIZATION)
	if err != nil {
		return
	}

	return
}

func getAutorizationResourceIds(ctx *gin.Context) (resourceIds []int, err error) {
	res, err := getAuthorizations(ctx)
	if err != nil {
		return
	}

	resourceIds = lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })

	return
}

func getAuthorizationIds(ctx *gin.Context) (authIds []*model.AuthorizationIds, err error) {
	resourceIds, err := getAutorizationResourceIds(ctx)
	if err != nil {
		handleRemoteErr(ctx, err)
		return
	}

	err = mysql.DB.Model(authIds).Where("resource_id IN ?", resourceIds).Find(&authIds).Error
	return
}

func hasAuthorization(ctx *gin.Context, sess *gsession.Session) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if sess.ShareId != 0 {
		return true
	}

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	if sess.Session.Asset == nil {
		if err := mysql.DB.Model(sess.Session.Asset).Where("id=?", sess.AssetId).First(&sess.Session.Asset).Error; err != nil {
			return
		}
	}

	authIds, err := getAuthorizationIds(ctx)
	if err != nil {
		return
	}
	if ok = lo.ContainsBy(authIds, func(item *model.AuthorizationIds) bool {
		return item.NodeId == 0 && item.AssetId == sess.AssetId && item.AccountId == sess.AccountId
	}); ok {
		return
	}
	ctx.Set(kAuthorizationIds, authIds)

	nodeIds, assetIds, accountIds := getIdsByAuthorizationIds(ctx)
	tmp, err := handleSelfChild(ctx, nodeIds...)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}
	nodeIds = append(nodeIds, tmp...)
	if ok = lo.Contains(nodeIds, sess.Session.Asset.ParentId) || lo.Contains(assetIds, sess.AssetId) || lo.Contains(accountIds, sess.AccountId); ok {
		return
	}

	ids, err := getAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}

	return lo.Contains(ids, sess.AssetId)
}
