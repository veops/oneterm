package service

import (
	"context"
	"errors"

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

// Define constants if they don't exist in the acl package
const (
	USE = "use" // Add USE constant, if it already exists in acl package, please use acl.USE
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
	HasAuthorization(ctx context.Context, sess *gsession.Session) bool
	GetAuthsByAsset(ctx context.Context, asset *model.Asset) ([]*model.Authorization, error)
	HandleAuthorization(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) error
	GetNodeAssetAccountIdsByAction(ctx context.Context, action string) (nodeIds, assetIds, accountIds []int, err error)
	GetAuthorizationIds(ctx context.Context) ([]*model.AuthorizationIds, error)
	GetIdsByAuthorizationIds(ctx context.Context) (nodeIds, assetIds, accountIds []int)
	GetAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) ([]int, error)
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

	if asset != nil && asset.Id > 0 {
		var pres []*model.Authorization
		pres, err = s.GetAuthsByAsset(ctx, asset)
		if err != nil {
			return
		}

		switch action {
		case model.ACTION_CREATE, model.ACTION_UPDATE:
			for _, v := range auths {
				if v == nil {
					continue
				}
				v.AssetId = asset.Id
				v.NodeId = 0
				v.SetUpdaterId(currentUser.GetUid())
				v.SetResourceId(v.ResourceId)
				if v.Id <= 0 {
					v.SetCreatorId(currentUser.GetUid())
				}
				if err = tx.Save(v).Error; err != nil {
					return
				}
			}
		case model.ACTION_DELETE:
			for _, v := range pres {
				if v == nil {
					continue
				}
				if err = tx.Delete(v).Error; err != nil {
					return
				}
			}
		}
		return
	}

	for _, v := range auths {
		if v == nil {
			continue
		}
		if v.Id == 0 && v.AssetId == 0 && v.AccountId == 0 && v.NodeId == 0 {
			err = errors.New("invalid authorization")
			return
		}
		if v.Id <= 0 {
			v.SetCreatorId(currentUser.GetUid())
		}
		v.SetUpdaterId(currentUser.GetUid())

		switch action {
		case model.ACTION_CREATE, model.ACTION_UPDATE:
			if err = tx.Save(v).Error; err != nil {
				return
			}
		case model.ACTION_DELETE:
			if err = tx.Delete(v).Error; err != nil {
				return
			}
		}
	}
	return
}

func (s *AuthorizationService) GetAuthorizationIds(ctx context.Context) (authIds []*model.AuthorizationIds, err error) {
	authIds, err = s.repo.GetAuthorizationIds(ctx)
	return
}

func (s *AuthorizationService) HasAuthorization(ctx context.Context, sess *gsession.Session) (ok bool) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if ok = acl.IsAdmin(currentUser); ok {
		return
	}

	nodeIds, assetIds, accountIds, err := s.GetNodeAssetAccountIdsByAction(ctx, USE)
	if err != nil {
		return
	}

	assetId := sess.Session.AssetId
	accountId := sess.Session.AccountId

	if assetId > 0 && accountId > 0 {
		if !lo.Contains(assetIds, assetId) {
			return
		}
		if !lo.Contains(accountIds, accountId) {
			ok = false
			authIds, err := s.repo.GetAuthorizationIdsByAssetAccount(ctx, assetId, accountId)
			if err != nil {
				logger.L().Error("get authrization failed", zap.Error(err))
				return
			}
			for _, v := range authIds {
				if v.NodeId > 0 && lo.Contains(nodeIds, v.NodeId) {
					ok = true
					break
				}
			}
			return
		}
		ok = true
		return
	}

	return
}

func (s *AuthorizationService) GetIdsByAuthorizationIds(ctx context.Context) (nodeIds, assetIds, accountIds []int) {
	authIds, err := s.GetAuthorizationIds(ctx)
	if err != nil {
		return
	}
	for _, v := range authIds {
		if v.NodeId > 0 {
			nodeIds = append(nodeIds, v.NodeId)
		}
		if v.AssetId > 0 {
			assetIds = append(assetIds, v.AssetId)
		}
		if v.AccountId > 0 {
			accountIds = append(accountIds, v.AccountId)
		}
	}
	return
}

func (s *AuthorizationService) GetAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) ([]int, error) {
	return s.repo.GetAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
}
