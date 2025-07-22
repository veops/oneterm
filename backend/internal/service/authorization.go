package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
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

	// Migrate Asset authorization
	if err := migrationService.MigrateV1ToV2(ctx); err != nil {
		logger.L().Error("Failed to migrate Asset authorization rules from V1 to V2", zap.Error(err))
		// Continue with service initialization even if migration fails
		// This allows the system to start with existing V2 rules
	}

	// Migrate Node authorization
	if err := MigrateNodeAuthorization(); err != nil {
		logger.L().Error("Failed to migrate Node authorization rules from V1 to V2", zap.Error(err))
		// Continue with service initialization even if migration fails
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
	HandleAuthorizationV2(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, node *model.Node, auths ...*model.Authorization) error
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
	return s.HandleAuthorizationV2(ctx, tx, action, asset, nil, auths...)
}

// HandleAuthorizationV2 handles authorization operations for both Asset and Node
func (s *AuthorizationService) HandleAuthorizationV2(ctx context.Context, tx *gorm.DB, action int, asset *model.Asset, node *model.Node, auths ...*model.Authorization) (err error) {
	defer repository.DeleteAllFromCacheDb(ctx, model.DefaultAuthorization)

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	eg := &errgroup.Group{}

	// Handle Asset authorization
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

	// Handle Node authorization
	if node != nil && node.Id > 0 {
		switch action {
		case model.ACTION_CREATE:
			// V2: Create authorization rules for node
			err = s.createV2AuthorizationRulesForNode(ctx, tx, node, currentUser)
			if err != nil {
				return err
			}
		case model.ACTION_DELETE:
			// V2: Delete authorization rules for this node
			err = s.deleteV2AuthorizationRulesForNode(ctx, tx, node)
			if err != nil {
				return err
			}
		case model.ACTION_UPDATE:
			// V2: Update authorization rules for this node
			err = s.updateV2AuthorizationRulesForNode(ctx, tx, node, currentUser)
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
		// Check if there's already an existing V2 rule for this asset-account combination
		// This handles cases where rules might exist from previous operations or historical data
		existingRule, err := s.findExistingV2Rule(tx, asset.Id, accountId)
		if err != nil {
			return fmt.Errorf("failed to check existing V2 rule: %w", err)
		}

		if existingRule != nil {
			// Found existing rule (either by JSON query or by name), update it instead of creating new one
			authDataCopy := authData
			if err := s.updateV2AuthorizationRuleById(ctx, tx, existingRule.Id, &authDataCopy, currentUser); err != nil {
				return fmt.Errorf("failed to update existing V2 rule %d for account %d: %w", existingRule.Id, accountId, err)
			}

			// Set the found RuleId in the asset.Authorization field
			authDataCopy.RuleId = existingRule.Id
			asset.Authorization[accountId] = authDataCopy

			// Update the asset's authorization field in database to include the rule ID
			if err := tx.Model(asset).Update("authorization", asset.Authorization).Error; err != nil {
				return fmt.Errorf("failed to update asset authorization with existing rule ID: %w", err)
			}
		} else {
			// No existing rule found, create new one
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
					// Log error but continue - ACL resource was created successfully
					logger.L().Error("Failed to grant role permissions during rule creation",
						zap.Int("assetId", asset.Id),
						zap.Int("accountId", accountId),
						zap.Int("resourceId", resourceId),
						zap.Ints("rids", authData.Rids),
						zap.Error(err))
				}
			}

			// Important: Update the asset.Authorization field with the rule ID
			authDataCopy := authData
			authDataCopy.RuleId = rule.Id
			asset.Authorization[accountId] = authDataCopy

			// Update the asset's authorization field in database to include the rule ID
			if err := tx.Model(asset).Update("authorization", asset.Authorization).Error; err != nil {
				return fmt.Errorf("failed to update asset authorization with rule ID: %w", err)
			}
		}
	}

	return nil
}

// deleteV2AuthorizationRulesForAsset deletes V2 authorization rules for an asset
func (s *AuthorizationService) deleteV2AuthorizationRulesForAsset(ctx context.Context, tx *gorm.DB, asset *model.Asset) error {
	// Delete rules based on RuleId stored in asset.Authorization field
	for accountId, authData := range asset.Authorization {
		if authData.RuleId > 0 {
			if err := s.deleteV2AuthorizationRuleById(ctx, tx, authData.RuleId); err != nil {
				// Log error but continue with other deletions
				logger.L().Error("Failed to delete V2 rule for asset",
					zap.Int("assetId", asset.Id),
					zap.Int("accountId", accountId),
					zap.Int("ruleId", authData.RuleId),
					zap.Error(err))
			}
		}
	}

	return nil
}

// updateV2AuthorizationRulesForAsset updates V2 authorization rules for an asset
func (s *AuthorizationService) updateV2AuthorizationRulesForAsset(ctx context.Context, tx *gorm.DB, asset *model.Asset, currentUser *acl.Session) error {
	// Get current asset from database to compare with new data
	var currentAsset model.Asset
	if err := tx.Where("id = ?", asset.Id).First(&currentAsset).Error; err != nil {
		return fmt.Errorf("failed to get current asset: %w", err)
	}

	// Create maps for efficient comparison
	currentAuthMap := make(map[int]*model.AccountAuthorization) // accountId -> current auth data
	for accountId, authData := range currentAsset.Authorization {
		currentAuthMap[accountId] = &authData
	}

	newAuthMap := make(map[int]*model.AccountAuthorization) // accountId -> new auth data
	for accountId, authData := range asset.Authorization {
		authDataCopy := authData
		newAuthMap[accountId] = &authDataCopy
	}

	// Process deletions: rules that exist in current but not in new
	for accountId, currentAuth := range currentAuthMap {
		if _, exists := newAuthMap[accountId]; !exists && currentAuth.RuleId > 0 {
			// Delete the specific V2 rule using RuleId
			if err := s.deleteV2AuthorizationRuleById(ctx, tx, currentAuth.RuleId); err != nil {
				return fmt.Errorf("failed to delete V2 rule %d for account %d: %w", currentAuth.RuleId, accountId, err)
			}
		}
	}

	// Process creations and updates
	for accountId, newAuth := range newAuthMap {
		currentAuth, exists := currentAuthMap[accountId]

		// Check if we need to create or find existing rule
		// This handles: 1) completely new authorization 2) historical data without RuleId
		if !exists || currentAuth.RuleId == 0 {
			// Before creating new rule, check if there's already an existing V2 rule
			// This handles historical data that doesn't have RuleId in asset.Authorization
			existingRule, err := s.findExistingV2Rule(tx, asset.Id, accountId)
			if err != nil {
				return fmt.Errorf("failed to check existing V2 rule: %w", err)
			}

			if existingRule != nil {
				// Found existing rule (either by JSON query or by name), update it
				if err := s.updateV2AuthorizationRuleById(ctx, tx, existingRule.Id, newAuth, currentUser); err != nil {
					return fmt.Errorf("failed to update existing V2 rule %d for account %d: %w", existingRule.Id, accountId, err)
				}

				// Set the found RuleId in the asset.Authorization field for future use
				newAuth.RuleId = existingRule.Id
				asset.Authorization[accountId] = *newAuth
			} else {
				// No existing rule found, create new one
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
					Permissions: *newAuth.Permissions,

					// Role IDs for ACL integration
					Rids: newAuth.Rids,

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
				if len(newAuth.Rids) > 0 {
					if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), newAuth.Rids, resourceId, []string{acl.READ}); err != nil {
						// Log error but continue - ACL resource was created successfully
						logger.L().Error("Failed to grant role permissions during rule creation",
							zap.Int("assetId", asset.Id),
							zap.Int("accountId", accountId),
							zap.Int("resourceId", resourceId),
							zap.Ints("rids", newAuth.Rids),
							zap.Error(err))
					}
				}

				// Update the asset.Authorization field with the new rule ID
				newAuth.RuleId = rule.Id
				asset.Authorization[accountId] = *newAuth
			}

		} else {
			// Update existing rule using RuleId (normal case with complete data)
			if err := s.updateV2AuthorizationRuleById(ctx, tx, currentAuth.RuleId, newAuth, currentUser); err != nil {
				return fmt.Errorf("failed to update V2 rule %d for account %d: %w", currentAuth.RuleId, accountId, err)
			}

			// Keep the existing RuleId in the asset.Authorization field
			newAuth.RuleId = currentAuth.RuleId
			asset.Authorization[accountId] = *newAuth
		}
	}

	// Update the asset's authorization field in database with updated rule IDs
	if err := tx.Model(asset).Update("authorization", asset.Authorization).Error; err != nil {
		return fmt.Errorf("failed to update asset authorization field: %w", err)
	}

	return nil
}

// findExistingV2Rule finds an existing V2 rule for the given asset and account combination
func (s *AuthorizationService) findExistingV2Rule(tx *gorm.DB, assetId, accountId int) (*model.AuthorizationV2, error) {
	var rule model.AuthorizationV2

	// First: Look for V2 rules that target this specific asset and account combination (JSON query)
	err := tx.Where(
		"asset_selector->>'$.type' = ? AND asset_selector->>'$.values' LIKE ? AND account_selector->>'$.type' = ? AND account_selector->>'$.values' LIKE ?",
		model.SelectorTypeIds,
		fmt.Sprintf("%%\"%d\"%%", assetId),
		model.SelectorTypeIds,
		fmt.Sprintf("%%\"%d\"%%", accountId),
	).First(&rule).Error

	if err == nil {
		return &rule, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Second: Try to find by rule name pattern (for historical data compatibility)
	// Historical quick authorization rules follow the pattern: Asset-{assetId}-Account-{accountId}
	ruleName := fmt.Sprintf("Asset-%d-Account-%d", assetId, accountId)
	err = tx.Where("name = ?", ruleName).First(&rule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No existing rule found
		}
		return nil, err
	}

	return &rule, nil
}

// deleteV2AuthorizationRuleById deletes a specific V2 authorization rule by ID
func (s *AuthorizationService) deleteV2AuthorizationRuleById(ctx context.Context, tx *gorm.DB, ruleId int) error {
	// Get the rule first to delete ACL resource
	var rule model.AuthorizationV2
	if err := tx.Where("id = ?", ruleId).First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Rule already deleted, no action needed
			return nil
		}
		return fmt.Errorf("failed to find V2 rule: %w", err)
	}

	// Delete ACL resource
	if err := acl.DeleteResource(ctx, 0, rule.ResourceId); err != nil {
		logger.L().Error("Failed to delete ACL resource", zap.Int("resourceId", rule.ResourceId), zap.Error(err))
		// Continue with database deletion even if ACL deletion fails
	}

	// Delete the rule from database
	if err := tx.Delete(&rule).Error; err != nil {
		return fmt.Errorf("failed to delete V2 authorization rule: %w", err)
	}

	return nil
}

// updateV2AuthorizationRuleById updates a specific V2 authorization rule by ID
func (s *AuthorizationService) updateV2AuthorizationRuleById(ctx context.Context, tx *gorm.DB, ruleId int, newAuth *model.AccountAuthorization, currentUser *acl.Session) error {
	// Get the existing rule
	var existingRule model.AuthorizationV2
	if err := tx.Where("id = ?", ruleId).First(&existingRule).Error; err != nil {
		return fmt.Errorf("failed to find V2 rule: %w", err)
	}

	needsUpdate := false

	// Check if permissions changed
	if !reflect.DeepEqual(existingRule.Permissions, *newAuth.Permissions) {
		existingRule.Permissions = *newAuth.Permissions
		needsUpdate = true
	}

	// Check if role IDs changed
	if !reflect.DeepEqual(existingRule.Rids, newAuth.Rids) {
		// Revoke old role permissions
		if len(existingRule.Rids) > 0 {
			if err := acl.BatchRevokeRoleResource(ctx, currentUser.GetUid(), existingRule.Rids, existingRule.ResourceId, []string{acl.READ}); err != nil {
				// Log error but continue - ACL resource might not exist
				logger.L().Error("Failed to revoke old role permissions, continuing anyway",
					zap.Int("ruleId", ruleId),
					zap.Int("resourceId", existingRule.ResourceId),
					zap.Ints("rids", existingRule.Rids),
					zap.Error(err))
			}
		}

		// Grant new role permissions
		if len(newAuth.Rids) > 0 {
			if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), newAuth.Rids, existingRule.ResourceId, []string{acl.READ}); err != nil {
				// Log error but continue - we can still update the database record
				logger.L().Error("Failed to grant new role permissions, continuing anyway",
					zap.Int("ruleId", ruleId),
					zap.Int("resourceId", existingRule.ResourceId),
					zap.Ints("rids", newAuth.Rids),
					zap.Error(err))
			}
		}

		existingRule.Rids = newAuth.Rids
		needsUpdate = true
	}

	// Update the rule if needed
	if needsUpdate {
		existingRule.UpdaterId = currentUser.GetUid()
		if err := tx.Model(&existingRule).Updates(map[string]interface{}{
			"permissions": existingRule.Permissions,
			"rids":        existingRule.Rids,
			"updater_id":  existingRule.UpdaterId,
		}).Error; err != nil {
			return fmt.Errorf("failed to update V2 authorization rule: %w", err)
		}
	}

	return nil
}

// createV2AuthorizationRulesForNode creates V2 authorization rules for a node
func (s *AuthorizationService) createV2AuthorizationRulesForNode(ctx context.Context, tx *gorm.DB, node *model.Node, currentUser *acl.Session) error {
	if len(node.Authorization) == 0 {
		return nil
	}

	for accountId, authData := range node.Authorization {
		// Check if there's already an existing V2 rule for this node-account combination
		// This handles cases where rules might exist from previous operations or historical data
		existingRule, err := s.findExistingV2RuleForNode(tx, node.Id, accountId)
		if err != nil {
			return fmt.Errorf("failed to check existing V2 rule: %w", err)
		}

		if existingRule != nil {
			// Found existing rule (either by JSON query or by name), update it instead of creating new one
			authDataCopy := authData
			if err := s.updateV2AuthorizationRuleById(ctx, tx, existingRule.Id, &authDataCopy, currentUser); err != nil {
				return fmt.Errorf("failed to update existing V2 rule %d for account %d: %w", existingRule.Id, accountId, err)
			}

			// Set the found RuleId in the node.Authorization field
			authDataCopy.RuleId = existingRule.Id
			node.Authorization[accountId] = authDataCopy

			// Update the node's authorization field in database to include the rule ID
			if err := tx.Model(node).Update("authorization", node.Authorization).Error; err != nil {
				return fmt.Errorf("failed to update node authorization with existing rule ID: %w", err)
			}
		} else {
			// No existing rule found, create new one
			rule := &model.AuthorizationV2{
				Name:        fmt.Sprintf("Node-%d-Account-%d", node.Id, accountId),
				Description: fmt.Sprintf("Auto-generated rule for node %s and account %d", node.Name, accountId),
				Enabled:     true,

				// Target selectors - specific node and account
				NodeSelector: model.TargetSelector{
					Type:       model.SelectorTypeIds,
					Values:     []string{fmt.Sprintf("%d", node.Id)},
					ExcludeIds: []int{},
				},
				AccountSelector: model.TargetSelector{
					Type:       model.SelectorTypeIds,
					Values:     []string{fmt.Sprintf("%d", accountId)},
					ExcludeIds: []int{},
				},

				// Use permissions from node.Authorization
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
					// Log error but continue - ACL resource was created successfully
					logger.L().Error("Failed to grant role permissions during rule creation",
						zap.Int("nodeId", node.Id),
						zap.Int("accountId", accountId),
						zap.Int("resourceId", resourceId),
						zap.Ints("rids", authData.Rids),
						zap.Error(err))
				}
			}

			// Important: Update the node.Authorization field with the rule ID
			authDataCopy := authData
			authDataCopy.RuleId = rule.Id
			node.Authorization[accountId] = authDataCopy

			// Update the node's authorization field in database to include the rule ID
			if err := tx.Model(node).Update("authorization", node.Authorization).Error; err != nil {
				return fmt.Errorf("failed to update node authorization with rule ID: %w", err)
			}
		}
	}

	return nil
}

// deleteV2AuthorizationRulesForNode deletes V2 authorization rules for a node
func (s *AuthorizationService) deleteV2AuthorizationRulesForNode(ctx context.Context, tx *gorm.DB, node *model.Node) error {
	// Delete rules based on RuleId stored in node.Authorization field
	for accountId, authData := range node.Authorization {
		if authData.RuleId > 0 {
			if err := s.deleteV2AuthorizationRuleById(ctx, tx, authData.RuleId); err != nil {
				// Log error but continue with other deletions
				logger.L().Error("Failed to delete V2 rule for node",
					zap.Int("nodeId", node.Id),
					zap.Int("accountId", accountId),
					zap.Int("ruleId", authData.RuleId),
					zap.Error(err))
			}
		}
	}

	return nil
}

// updateV2AuthorizationRulesForNode updates V2 authorization rules for a node
func (s *AuthorizationService) updateV2AuthorizationRulesForNode(ctx context.Context, tx *gorm.DB, node *model.Node, currentUser *acl.Session) error {
	// Get current node from database to compare with new data
	var currentNode model.Node
	if err := tx.Where("id = ?", node.Id).First(&currentNode).Error; err != nil {
		return fmt.Errorf("failed to get current node: %w", err)
	}

	// Create maps for efficient comparison
	currentAuthMap := make(map[int]*model.AccountAuthorization) // accountId -> current auth data
	for accountId, authData := range currentNode.Authorization {
		currentAuthMap[accountId] = &authData
	}

	newAuthMap := make(map[int]*model.AccountAuthorization) // accountId -> new auth data
	for accountId, authData := range node.Authorization {
		authDataCopy := authData
		newAuthMap[accountId] = &authDataCopy
	}

	// Process deletions: rules that exist in current but not in new
	for accountId, currentAuth := range currentAuthMap {
		if _, exists := newAuthMap[accountId]; !exists && currentAuth.RuleId > 0 {
			// Delete the specific V2 rule using RuleId
			if err := s.deleteV2AuthorizationRuleById(ctx, tx, currentAuth.RuleId); err != nil {
				return fmt.Errorf("failed to delete V2 rule %d for account %d: %w", currentAuth.RuleId, accountId, err)
			}
		}
	}

	// Process creations and updates
	for accountId, newAuth := range newAuthMap {
		currentAuth, exists := currentAuthMap[accountId]

		// Check if we need to create or find existing rule
		// This handles: 1) completely new authorization 2) historical data without RuleId
		if !exists || currentAuth.RuleId == 0 {
			// Before creating new rule, check if there's already an existing V2 rule
			// This handles historical data that doesn't have RuleId in node.Authorization
			existingRule, err := s.findExistingV2RuleForNode(tx, node.Id, accountId)
			if err != nil {
				return fmt.Errorf("failed to check existing V2 rule: %w", err)
			}

			if existingRule != nil {
				// Found existing rule (either by JSON query or by name), update it
				if err := s.updateV2AuthorizationRuleById(ctx, tx, existingRule.Id, newAuth, currentUser); err != nil {
					return fmt.Errorf("failed to update existing V2 rule %d for account %d: %w", existingRule.Id, accountId, err)
				}

				// Set the found RuleId in the node.Authorization field for future use
				newAuth.RuleId = existingRule.Id
				node.Authorization[accountId] = *newAuth
			} else {
				// No existing rule found, create new one
				rule := &model.AuthorizationV2{
					Name:        fmt.Sprintf("Node-%d-Account-%d", node.Id, accountId),
					Description: fmt.Sprintf("Auto-generated rule for node %s and account %d", node.Name, accountId),
					Enabled:     true,

					// Target selectors - specific node and account
					NodeSelector: model.TargetSelector{
						Type:       model.SelectorTypeIds,
						Values:     []string{fmt.Sprintf("%d", node.Id)},
						ExcludeIds: []int{},
					},
					AccountSelector: model.TargetSelector{
						Type:       model.SelectorTypeIds,
						Values:     []string{fmt.Sprintf("%d", accountId)},
						ExcludeIds: []int{},
					},

					// Use permissions from node.Authorization
					Permissions: *newAuth.Permissions,

					// Role IDs for ACL integration
					Rids: newAuth.Rids,

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
				if len(newAuth.Rids) > 0 {
					if err := acl.BatchGrantRoleResource(ctx, currentUser.GetUid(), newAuth.Rids, resourceId, []string{acl.READ}); err != nil {
						// Log error but continue - ACL resource was created successfully
						logger.L().Error("Failed to grant role permissions during rule creation",
							zap.Int("nodeId", node.Id),
							zap.Int("accountId", accountId),
							zap.Int("resourceId", resourceId),
							zap.Ints("rids", newAuth.Rids),
							zap.Error(err))
					}
				}

				// Update the node.Authorization field with the new rule ID
				newAuth.RuleId = rule.Id
				node.Authorization[accountId] = *newAuth
			}

		} else {
			// Update existing rule using RuleId (normal case with complete data)
			if err := s.updateV2AuthorizationRuleById(ctx, tx, currentAuth.RuleId, newAuth, currentUser); err != nil {
				return fmt.Errorf("failed to update V2 rule %d for account %d: %w", currentAuth.RuleId, accountId, err)
			}

			// Keep the existing RuleId in the node.Authorization field
			newAuth.RuleId = currentAuth.RuleId
			node.Authorization[accountId] = *newAuth
		}
	}

	// Update the node's authorization field in database with updated rule IDs
	if err := tx.Model(node).Update("authorization", node.Authorization).Error; err != nil {
		return fmt.Errorf("failed to update node authorization field: %w", err)
	}

	return nil
}

// findExistingV2RuleForNode finds an existing V2 rule for the given node and account combination
func (s *AuthorizationService) findExistingV2RuleForNode(tx *gorm.DB, nodeId, accountId int) (*model.AuthorizationV2, error) {
	var rule model.AuthorizationV2

	// First: Look for V2 rules that target this specific node and account combination (JSON query)
	err := tx.Where(
		"node_selector->>'$.type' = ? AND node_selector->>'$.values' LIKE ? AND account_selector->>'$.type' = ? AND account_selector->>'$.values' LIKE ?",
		model.SelectorTypeIds,
		fmt.Sprintf("%%\"%d\"%%", nodeId),
		model.SelectorTypeIds,
		fmt.Sprintf("%%\"%d\"%%", accountId),
	).First(&rule).Error

	if err == nil {
		return &rule, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Second: Try to find by rule name pattern (for historical data compatibility)
	// Historical quick authorization rules follow the pattern: Node-{nodeId}-Account-{accountId}
	ruleName := fmt.Sprintf("Node-%d-Account-%d", nodeId, accountId)
	err = tx.Where("name = ?", ruleName).First(&rule).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No existing rule found
		}
		return nil, err
	}

	return &rule, nil
}
