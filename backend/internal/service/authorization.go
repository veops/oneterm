package service

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	kAuthorizationIds = "authorizationIds"
)

var (
	// Global service instance, created at initialization
	DefaultAuthService IAuthorizationService
)

// InitAuthorizationService initializes the global authorization service
func InitAuthorizationService() {
	repo := repository.NewAuthorizationRepository(dbpkg.DB)
	DefaultAuthService = NewAuthorizationService(repo, dbpkg.DB)
}

type IAuthorizationService interface {
	UpsertAuthorization(ctx context.Context, auth *model.Authorization) error
	UpsertAuthorizationWithTx(ctx context.Context, auth *model.Authorization) error
	DeleteAuthorization(ctx context.Context, auth *model.Authorization) error
	GetAuthorizations(ctx context.Context, nodeId, assetId, accountId int) ([]*model.Authorization, int64, error)
	GetAuthorizationById(ctx context.Context, id int) (*model.Authorization, error)
	HasPermAuthorization(ctx context.Context, auth *model.Authorization, action string) bool
	HasAuthorization(ctx *gin.Context, sess *gsession.Session) (bool, error)
	GetAuthsByAsset(ctx context.Context, asset *model.Asset) ([]*model.Authorization, error)
	HandleAuthorization(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) error
	GetNodeAssetAccountIdsByAction(ctx context.Context, action string) (nodeIds, assetIds, accountIds []int, err error)
	GetAuthorizationIds(ctx *gin.Context) ([]*model.AuthorizationIds, error)
}

type AuthorizationService struct {
	repo repository.IAuthorizationRepository
	db   *gorm.DB // Add database field for transaction processing
}

func NewAuthorizationService(repo repository.IAuthorizationRepository, db *gorm.DB) IAuthorizationService {
	return &AuthorizationService{
		repo: repo,
		db:   db,
	}
}

// UpsertAuthorization updates or creates authorization (without transaction)
func (s *AuthorizationService) UpsertAuthorization(ctx context.Context, auth *model.Authorization) error {
	return s.repo.UpsertAuthorization(ctx, auth)
}

// UpsertAuthorizationWithTx updates or creates authorization (with transaction)
func (s *AuthorizationService) UpsertAuthorizationWithTx(ctx context.Context, auth *model.Authorization) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Create a Repository in the transaction
		txRepo := repository.NewAuthorizationRepository(tx)

		// Check if it exists
		existing, err := txRepo.GetAuthorizationByFields(ctx, auth.NodeId, auth.AssetId, auth.AccountId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if existing != nil {
			auth.Id = existing.Id
			auth.ResourceId = existing.ResourceId
		}

		// Determine action based on whether it's an update or create
		action := lo.Ternary(auth.Id > 0, model.ACTION_UPDATE, model.ACTION_CREATE)

		// Create a temporary Service for transaction handling
		txService := &AuthorizationService{repo: txRepo, db: s.db}

		return txService.HandleAuthorization(ctx, tx, action, nil, auth)
	})
}

// GetAuthorizationById gets authorization by ID
func (s *AuthorizationService) GetAuthorizationById(ctx context.Context, id int) (*model.Authorization, error) {
	return s.repo.GetAuthorizationById(ctx, id)
}

func (s *AuthorizationService) DeleteAuthorization(ctx context.Context, auth *model.Authorization) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := repository.NewAuthorizationRepository(tx)
		txService := &AuthorizationService{repo: txRepo, db: s.db}
		return txService.HandleAuthorization(ctx, tx, model.ACTION_DELETE, nil, auth)
	})
}

func (s *AuthorizationService) GetAuthorizations(ctx context.Context, nodeId, assetId, accountId int) ([]*model.Authorization, int64, error) {
	return s.repo.GetAuthorizations(ctx, nodeId, assetId, accountId)
}

func (s *AuthorizationService) GetNodeAssetAccountIdsByAction(ctx context.Context, action string) (nodeIds, assetIds, accountIds []int, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}
	ch := make(chan bool)

	eg.Go(func() (err error) {
		defer close(ch)
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
		if err != nil {
			return
		}
		nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(resIds, n.ResourceId) })
		nodeIds = lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })
		nodeIds, err = repository.HandleSelfChild(ctx, nodeIds...)
		return
	})

	eg.Go(func() (err error) {
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		<-ch
		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
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
		res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_ACCOUNT)
		if err != nil {
			return
		}
		res = lo.Filter(res, func(r *acl.Resource, _ int) bool { return lo.Contains(r.Permissions, action) })
		resIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
		accounts, err := repository.GetAllFromCacheDb(ctx, model.DefaultAccount)
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

func (s *AuthorizationService) HasPermAuthorization(ctx context.Context, auth *model.Authorization, action string) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	if auth == nil {
		auth = &model.Authorization{}
	}

	nodeIds, assetIds, accountIds, err := s.GetNodeAssetAccountIdsByAction(ctx, action)
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

func (s *AuthorizationService) GetAuthsByAsset(ctx context.Context, asset *model.Asset) ([]*model.Authorization, error) {
	auths, err := s.repo.GetAuthsByAsset(ctx, asset)
	return auths, err
}

func (s *AuthorizationService) HandleAuthorization(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, model.DefaultAuthorization)

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}

	if asset != nil && asset.Id > 0 {
		var pres []*model.Authorization
		pres, err = s.GetAuthsByAsset(ctx, asset)
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

						if err = tx.Delete(p).Error; err != nil {
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
				if resourceId, err = acl.CreateAcl(ctx, currentUser, config.RESOURCE_AUTHORIZATION, auth.GetName()); err != nil {
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
				pre, err := s.GetAuthorizationById(ctx, auth.GetId())
				if err != nil {
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						return
					}
					resourceId := 0
					if resourceId, err = acl.CreateAcl(ctx, currentUser, config.RESOURCE_AUTHORIZATION, auth.GetName()); err != nil {
						return
					}
					auth.ResourceId = resourceId
					if err = tx.Create(auth).Error; err != nil {
						return
					}
					pre = &model.Authorization{Rids: []int{}}
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

	res, err = acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_AUTHORIZATION)
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

func (s *AuthorizationService) GetAuthorizationIds(ctx *gin.Context) (authIds []*model.AuthorizationIds, err error) {
	resourceIds, err := getAutorizationResourceIds(ctx)
	if err != nil {
		return
	}
	authIds, err = s.repo.GetAuthorizationIds(ctx, resourceIds)
	return
}

func (s *AuthorizationService) HasAuthorization(ctx *gin.Context, sess *gsession.Session) (ok bool, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if sess.ShareId != 0 {
		return true, nil
	}

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	if sess.Session.Asset == nil {
		if err := s.db.Model(sess.Session.Asset).Where("id=?", sess.AssetId).First(&sess.Session.Asset).Error; err != nil {
			return false, err
		}
	}

	authIds, err := s.GetAuthorizationIds(ctx)
	if err != nil {
		return false, err
	}
	if ok = lo.ContainsBy(authIds, func(item *model.AuthorizationIds) bool {
		return item.NodeId == 0 && item.AssetId == sess.AssetId && item.AccountId == sess.AccountId
	}); ok {
		return true, nil
	}
	ctx.Set(kAuthorizationIds, authIds)

	authorizationIds, ok := ctx.Value(kAuthorizationIds).([]*model.AuthorizationIds)
	if !ok || len(authorizationIds) == 0 {
		return false, errors.New("authorizationIds not found")
	}
	assetService := NewAssetService()
	nodeIds, assetIds, accountIds := assetService.GetIdsByAuthorizationIds(ctx, authorizationIds)
	tmp, err := repository.HandleSelfChild(ctx, nodeIds...)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return false, err
	}
	nodeIds = append(nodeIds, tmp...)
	if ok = lo.Contains(nodeIds, sess.Session.Asset.ParentId) || lo.Contains(assetIds, sess.AssetId) || lo.Contains(accountIds, sess.AccountId); ok {
		return true, nil
	}

	ids, err := assetService.GetAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
	if err != nil {
		logger.L().Error("", zap.Error(err))
		return false, err
	}

	return lo.Contains(ids, sess.AssetId), nil

}
