package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

func HandleAuthorization(currentUser *acl.Session, tx *gorm.DB, action int, old, new *model.Asset) (err error) {
	ctx := context.Background()
	assetId := lo.TernaryF(new == nil, func() int { return old.Id }, func() int { return new.Id })
	mtx := &sync.Mutex{}
	eg := &errgroup.Group{}
	if action == model.ACTION_UPDATE {
		if sameAuthorization(old.Authorization, new.Authorization) {
			return
		}
		for id := range old.Authorization {
			if _, ok := new.Authorization[id]; ok {
				continue
			}
			accountId := id
			eg.Go(func() (err error) {
				a := &model.Authorization{}
				if err = mysql.DB.
					Model(a).
					Select("id", "resource_id").
					Where("asset_id = ? AND account_id = ?", assetId, accountId).
					First(a).
					Error; err != nil {
					return
				}
				if err = acl.DeleteResource(ctx, currentUser.GetUid(), a.ResourceId); err != nil {
					return
				}
				mtx.Lock()
				defer mtx.Unlock()
				err = tx.Delete(a, a.Id).Error
				return
			})
		}
	}
	as := lo.TernaryF(action == model.ACTION_DELETE,
		func() model.Map[int, model.Slice[int]] { return old.Authorization },
		func() model.Map[int, model.Slice[int]] { return new.Authorization })
	for k, v := range as {
		accountId := k
		curRids := lo.Uniq(v)
		eg.Go(func() (err error) {
			resourceId := 0
			if err = mysql.DB.
				Model(&model.Authorization{}).
				Select("resource_id").
				Where("asset_id = ? AND account_id = ?", assetId, accountId).
				First(&resourceId).
				Error; err != nil {
				notFount := errors.Is(err, gorm.ErrRecordNotFound)
				if !notFount || (notFount && action == model.ACTION_DELETE) {
					return
				}
				if resourceId, err = acl.CreateGrantAcl(ctx, currentUser, conf.GetResourceTypeName(conf.RESOURCE_AUTHORIZATION),
					fmt.Sprintf("%d-%d", assetId, accountId)); err != nil {
					return
				}
				mtx.Lock()
				if err = tx.Create(&model.Authorization{AssetId: assetId, AccountId: accountId, ResourceId: resourceId,
					CreatorId: currentUser.GetUid(), UpdaterId: currentUser.GetUid()}).Error; err != nil {
					return
				}
				mtx.Unlock()
			}
			switch action {
			case model.ACTION_CREATE:
				err = acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), curRids, resourceId, []string{acl.READ})
			case model.ACTION_DELETE:
				err = acl.DeleteResource(ctx, currentUser.GetUid(), resourceId)
			case model.ACTION_UPDATE:
				var res map[string]*acl.ResourcePermissionsRespItem
				res, err = acl.GetResourcePermissions(ctx, resourceId)
				if err != nil {
					return
				}
				perms := make([]*acl.Perm, 0)
				for _, v := range res {
					perms = append(perms, v.Perms...)
				}
				preRids := lo.Map(lo.Filter(perms, func(p *acl.Perm, _ int) bool { return p.Name == acl.READ }), func(p *acl.Perm, _ int) int { return p.Rid })
				revokeRids := lo.Without(preRids, curRids...)
				if len(revokeRids) > 0 {
					if err = acl.BatchRevokeRoleResource(ctx, currentUser.GetUid(), revokeRids, resourceId, []string{acl.READ}); err != nil {
						return
					}
				}
				grantRids := lo.Without(curRids, preRids...)
				if len(grantRids) > 0 {
					err = acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), grantRids, resourceId, []string{acl.READ})
				}
				return
			}
			return
		})
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

func GetAutorizationResourceIds(ctx *gin.Context) (resourceIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	var rs []*acl.Resource
	rs, err = acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.RESOURCE_AUTHORIZATION)
	if err != nil {
		return
	}
	resourceIds = lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId })

	return
}

func HasAuthorization(ctx *gin.Context) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	rs, err := acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.RESOURCE_AUTHORIZATION)
	if err != nil {
		logger.L().Error("check authorization failed", zap.Error(err))
		return
	}
	k := fmt.Sprintf("%s-%s", ctx.Param("asset_id"), ctx.Param("account_id"))
	_, ok = lo.Find(rs, func(r *acl.Resource) bool { return k == r.Name })
	return
}
