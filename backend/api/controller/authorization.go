package controller

import (
	"net/http"
	"reflect"
	"sort"

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
)

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

	for _, auth := range lo.Filter(auths, func(item *model.Authorization, _ int) bool { return item != nil }) {
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

func sameAuthorization(old, new model.Map[int, model.Slice[int]]) bool {
	if len(old) != len(new) {
		return false
	}
	ks := lo.Uniq(append(lo.Keys(old), lo.Keys(new)...))
	for _, k := range ks {
		if len(old[k]) != len(new[k]) {
			return false
		}
		o, n := make([]int, 0, len(old[k])), make([]int, 0, len(new[k]))
		copy(o, old[k])
		copy(n, new[k])
		sort.Ints(o)
		sort.Ints(n)
		if !reflect.DeepEqual(o, n) {
			return false
		}
	}
	return true
}

func getAutorizationResourceIds(ctx *gin.Context) (resourceIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	var rs []*acl.Resource
	rs, err = acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.RESOURCE_AUTHORIZATION)
	if err != nil {
		return
	}
	resourceIds = lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId })

	return
}

func hasAuthorization(ctx *gin.Context, assetId, accountId int) (ok bool) {
	if cast.ToString(ctx.Value("shareId")) != "" {
		return true
	}
	ids, err := getAutorizationResourceIds(ctx)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}
	cnt := int64(0)
	err = mysql.DB.Model(&model.Authorization{}).
		Where("asset_id =? AND account_id =? AND resource_id IN (?)", assetId, accountId, ids).
		Count(&cnt).Error
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return
	}

	return cnt > 0
}

// CreateAccount godoc
//
//	@Tags		authorization
//	@Param		authorization	body		model.Authorization	true	"authorization"
//	@Success	200		{object}	HttpResponse
//	@Router		/authorization [post]
func (c *Controller) CreateAuthorization(ctx *gin.Context) {
	auth := &model.Authorization{}
	err := ctx.ShouldBindBodyWithJSON(auth)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	if err := handleAuthorization(ctx, mysql.DB, model.ACTION_CREATE, nil, auth); err != nil {
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

// UpdateAccount godoc
//
//	@Tags		authorization
//	@Param		id		path		int				true	"authorization id"
//	@Param		authorization	body		model.Authorization	true	"authorization"
//	@Success	200		{object}	HttpResponse
//	@Router		/authorization/:id [put]
func (c *Controller) UpdateAuthorization(ctx *gin.Context) {
	auth := &model.Authorization{}
	err := ctx.ShouldBindBodyWithJSON(auth)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	auth.Id = cast.ToInt(ctx.Param("id"))
	if err := handleAuthorization(ctx, mysql.DB, model.ACTION_UPDATE, nil, auth); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": auth.GetId(),
		},
	})
}
