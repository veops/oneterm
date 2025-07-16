package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

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
	v2Repo := repository.NewAuthorizationV2Repository(dbpkg.DB)
	matcher := NewAuthorizationMatcher(v2Repo)

	// Perform V1 to V2 migration if needed
	migrationService := NewAuthorizationMigrationService(dbpkg.DB, repo, v2Repo)
	ctx := context.Background()
	if err := migrationService.MigrateV1ToV2(ctx); err != nil {
		logger.L().Error("Failed to migrate V1 authorization rules to V2", zap.Error(err))
		// Continue with service initialization even if migration fails
		// This allows the system to start with existing V2 rules
	}

	DefaultAuthService = NewAuthorizationService(repo, dbpkg.DB, matcher) // Use V2 by default
}

type IAuthorizationService interface {
	// V1 methods
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

	// V2 methods
	HasAuthorizationV2(ctx *gin.Context, sess *gsession.Session, actions ...model.AuthAction) (*model.BatchAuthResult, error)
	CheckPermission(ctx *gin.Context, nodeId, assetId, accountId int, action model.AuthAction) (*model.AuthResult, error)
}

type AuthorizationService struct {
	repo    repository.IAuthorizationRepository
	matcher IAuthorizationMatcher
	db      *gorm.DB
}

func NewAuthorizationService(repo repository.IAuthorizationRepository, db *gorm.DB, matcher IAuthorizationMatcher) IAuthorizationService {
	return &AuthorizationService{
		repo:    repo,
		matcher: matcher,
		db:      db,
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
		txService := &AuthorizationService{repo: txRepo, db: s.db, matcher: s.matcher}

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
		txService := &AuthorizationService{repo: txRepo, db: s.db, matcher: s.matcher}
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

// HandleAuthorization handles authorization operations
func (s *AuthorizationService) HandleAuthorization(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, auths ...*model.Authorization) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, model.DefaultAuthorization)

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}

	if asset != nil && asset.Id > 0 {
		switch action {
		case model.ACTION_CREATE:
			// V2: Create authorization rules instead of V1 authorization records
			err = s.createV2AuthorizationRulesForAsset(ctx, tx, asset, currentUser)
			if err != nil {
				return err
			}
		case model.ACTION_DELETE:
			// V2: Delete authorization rules for this asset
			err = s.deleteV2AuthorizationRulesForAsset(ctx, tx, asset)
			if err != nil {
				return err
			}
		case model.ACTION_UPDATE:
			// V2: Update authorization rules for this asset
			err = s.updateV2AuthorizationRulesForAsset(ctx, tx, asset, currentUser)
			if err != nil {
				return err
			}
		}
	}

	// Handle individual authorization records (V1 compatibility)
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

// HasAuthorization checks if the current user has permission to connect to the specified asset with the given account.
func (s *AuthorizationService) HasAuthorization(ctx *gin.Context, sess *gsession.Session) (ok bool, err error) {
	result, err := s.HasAuthorizationV2(ctx, sess, model.ActionConnect)
	if err != nil {
		return false, err
	}
	// Check if connect action is allowed in the batch result
	return result.IsAllowed(model.ActionConnect), nil
}

// HasAuthorizationV2 implements the new V2 authorization logic
func (s *AuthorizationService) HasAuthorizationV2(ctx *gin.Context, sess *gsession.Session, actions ...model.AuthAction) (*model.BatchAuthResult, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Helper function to create batch result for all actions
	createBatchResult := func(allowed bool, reason string) *model.BatchAuthResult {
		results := make(map[model.AuthAction]*model.AuthResult)
		for _, action := range actions {
			results[action] = &model.AuthResult{
				Allowed: allowed,
				Reason:  reason,
			}
		}
		return &model.BatchAuthResult{
			Results: results,
		}
	}

	// 1. Share sessions are always allowed
	if sess.ShareId != 0 {
		return createBatchResult(true, "Share session"), nil
	}

	// 2. Administrators have access to all resources
	if acl.IsAdmin(currentUser) {
		return createBatchResult(true, "Administrator access"), nil
	}

	// 3. Get user's authorized V2 rule IDs from ACL (like V1's AuthorizationIds)
	authV2ResourceIds, err := s.getAuthorizedV2ResourceIds(ctx)
	if err != nil {
		return createBatchResult(false, "Failed to get authorized rules"), err
	}

	if len(authV2ResourceIds) == 0 {
		return createBatchResult(false, "No authorization rules available"), nil
	}

	// Load asset if not already loaded
	if sess.Session.Asset == nil {
		if err := s.db.Model(sess.Session.Asset).Where("id=?", sess.AssetId).First(&sess.Session.Asset).Error; err != nil {
			return createBatchResult(false, "Asset not found"), err
		}
	}

	// Create base authorization request (without action, will be added per action)
	clientIP := s.getClientIP(ctx)
	baseReq := &model.BatchAuthRequest{
		UserId:    currentUser.GetUid(),
		NodeId:    sess.Session.Asset.ParentId,
		AssetId:   sess.AssetId,
		AccountId: sess.AccountId,
		Actions:   actions,
		ClientIP:  clientIP,
		Timestamp: time.Now(),
	}

	// Use V2 matcher with filtered rule scope
	return s.matcher.MatchBatchWithScope(ctx, baseReq, authV2ResourceIds)
}

// getAuthorizedV2ResourceIds gets V2 authorization rule resource IDs that user has permission to (like V1's AuthorizationIds)
func (s *AuthorizationService) getAuthorizedV2ResourceIds(ctx *gin.Context) ([]int, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get ACL resources for authorization_v2 that this user's role has access to
	res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_AUTHORIZATION)
	if err != nil {
		return nil, err
	}

	// Extract resource IDs (these are the V2 rule IDs user can access)
	resourceIds := lo.Map(res, func(r *acl.Resource, _ int) int { return r.ResourceId })
	return resourceIds, nil
}

// CheckPermission checks permission for specific node/asset/account combination
func (s *AuthorizationService) CheckPermission(ctx *gin.Context, nodeId, assetId, accountId int, action model.AuthAction) (*model.AuthResult, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all resources
	if acl.IsAdmin(currentUser) {
		return &model.AuthResult{
			Allowed: true,
			Reason:  "Administrator access",
		}, nil
	}

	// Create authorization request
	clientIP := s.getClientIP(ctx)
	req := &model.AuthRequest{
		UserId:    currentUser.GetUid(),
		NodeId:    nodeId,
		AssetId:   assetId,
		AccountId: accountId,
		Action:    action,
		ClientIP:  clientIP,
		Timestamp: time.Now(),
	}

	// Use V2 matcher
	return s.matcher.Match(ctx, req)
}

// getClientIP extracts client IP from gin context
func (s *AuthorizationService) getClientIP(ctx *gin.Context) string {
	// Try to get real IP from headers first
	clientIP := ctx.GetHeader("X-Forwarded-For")
	if clientIP == "" {
		clientIP = ctx.GetHeader("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = ctx.ClientIP()
	}

	// Parse and validate IP
	if ip := net.ParseIP(clientIP); ip != nil {
		return clientIP
	}

	return ""
}

// createV2AuthorizationRulesForAsset creates V2 authorization rules for an asset
func (s *AuthorizationService) createV2AuthorizationRulesForAsset(ctx context.Context, tx *gorm.DB, asset *model.Asset, currentUser *acl.Session) error {
	if len(asset.Authorization) == 0 {
		return nil
	}

	for accountId, authData := range asset.Authorization {
		// Create a V2 authorization rule for this asset-account combination
		rule := &model.AuthorizationV2{
			Name:        fmt.Sprintf("Asset-%d-Account-%d", asset.Id, accountId),
			Description: fmt.Sprintf("Auto-generated rule for asset %s and account %d", asset.Name, accountId),
			Enabled:     true,

			// Target selectors - specific asset and account
			AssetSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", asset.Id)},
				ExcludeIds: []int{},
			},
			AccountSelector: model.TargetSelector{
				Type:       model.SelectorTypeIds,
				Values:     []string{fmt.Sprintf("%d", accountId)},
				ExcludeIds: []int{},
			},

			// Use permissions from asset.Authorization
			Permissions: *authData.Permissions,

			// Role IDs for ACL integration
			Rids: authData.Rids,

			// Standard fields
			CreatorId: currentUser.GetUid(),
			UpdaterId: currentUser.GetUid(),
		}

		// Create ACL resource for this rule
		resourceId, err := acl.CreateAcl(ctx, currentUser, config.RESOURCE_AUTHORIZATION, rule.Name)
		if err != nil {
			return fmt.Errorf("failed to create ACL resource: %w", err)
		}
		rule.ResourceId = resourceId

		// Create the V2 rule
		if err := tx.Create(rule).Error; err != nil {
			return fmt.Errorf("failed to create V2 authorization rule: %w", err)
		}

		// Grant permissions to roles
		if len(authData.Rids) > 0 {
			if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), authData.Rids, resourceId, []string{acl.READ}); err != nil {
				return fmt.Errorf("failed to grant role permissions: %w", err)
			}
		}
	}

	return nil
}

// deleteV2AuthorizationRulesForAsset deletes V2 authorization rules for an asset
func (s *AuthorizationService) deleteV2AuthorizationRulesForAsset(ctx context.Context, tx *gorm.DB, asset *model.Asset) error {
	// Find all V2 rules that target this specific asset
	var rules []*model.AuthorizationV2
	if err := tx.Where("asset_selector->>'$.values' LIKE ?", fmt.Sprintf("%%\"%d\"%%", asset.Id)).Find(&rules).Error; err != nil {
		return fmt.Errorf("failed to find V2 rules for asset: %w", err)
	}

	// Delete each rule and its ACL resource
	for _, rule := range rules {
		// Delete ACL resource
		if err := acl.DeleteResource(ctx, 0, rule.ResourceId); err != nil {
			logger.L().Error("Failed to delete ACL resource", zap.Int("resourceId", rule.ResourceId), zap.Error(err))
			// Continue with database deletion even if ACL deletion fails
		}

		// Delete the rule from database
		if err := tx.Delete(rule).Error; err != nil {
			return fmt.Errorf("failed to delete V2 authorization rule: %w", err)
		}
	}

	return nil
}

// updateV2AuthorizationRulesForAsset updates V2 authorization rules for an asset
func (s *AuthorizationService) updateV2AuthorizationRulesForAsset(ctx context.Context, tx *gorm.DB, asset *model.Asset, currentUser *acl.Session) error {
	// For simplicity, we'll delete existing rules and create new ones
	// This ensures consistency and handles complex permission changes

	// First, delete existing rules for this asset
	if err := s.deleteV2AuthorizationRulesForAsset(ctx, tx, asset); err != nil {
		return fmt.Errorf("failed to delete existing V2 rules: %w", err)
	}

	// Then create new rules based on current asset.Authorization
	if err := s.createV2AuthorizationRulesForAsset(ctx, tx, asset, currentUser); err != nil {
		return fmt.Errorf("failed to create new V2 rules: %w", err)
	}

	return nil
}
