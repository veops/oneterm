package service

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/config"
	"gorm.io/gorm"
)

var (
	ErrAccountNotFound  = errors.New("account not found")
	ErrAccountForbidden = errors.New("permission denied")
)

// AccountService handles account business logic
type AccountService struct {
	repo repository.AccountRepository
}

// NewAccountService creates a new account service
func NewAccountService() *AccountService {
	return &AccountService{
		repo: repository.NewAccountRepository(),
	}
}

// AttachAssetCount attaches asset count to accounts
func (s *AccountService) AttachAssetCount(ctx context.Context, accounts []*model.Account) error {
	return s.repo.AttachAssetCount(ctx, accounts)
}

// CheckAssetDependencies checks if account has dependent assets
func (s *AccountService) CheckAssetDependencies(ctx context.Context, id int) (string, error) {
	return s.repo.CheckAssetDependencies(ctx, id)
}

// BuildQuery constructs account query with basic filters
func (s *AccountService) BuildQuery(ctx *gin.Context) *gorm.DB {
	return s.repo.BuildQuery(ctx)
}

// GetAccountIdsByAuthorization gets account IDs by authorization
func (s *AccountService) GetAccountIdsByAuthorization(ctx context.Context, assetIds []int, authorizationIds []int) ([]int, error) {
	return s.repo.GetAccountIdsByAuthorization(ctx, assetIds, authorizationIds)
}

// BuildQueryWithAuthorization builds query with integrated V2 authorization filter
func (s *AccountService) BuildQueryWithAuthorization(ctx *gin.Context) (*gorm.DB, error) {
	// Start with base query
	db := s.repo.BuildQuery(ctx)

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all accounts
	if acl.IsAdmin(currentUser) {
		return db, nil
	}

	// Apply V2 authorization filter: get authorized account IDs using V2 system
	authV2Service := NewAuthorizationV2Service()
	_, _, accountIds, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by authorized account IDs at database level (much more efficient)
	if len(accountIds) == 0 {
		// No access to any accounts
		db = db.Where("1 = 0") // Returns empty result set efficiently
	} else {
		db = db.Where("id IN ?", accountIds)
	}

	return db, nil
}

// GetAccountCredentials gets account credentials with ACL permission check
func (s *AccountService) GetAccountCredentials(ctx *gin.Context, accountId int) (*model.Account, error) {
	currentUser, err := acl.GetSessionFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	var account model.Account
	baseRepo := repository.NewBaseRepository()
	if err := baseRepo.GetById(ctx, accountId, &account); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	if !acl.IsAdmin(currentUser) {
		hasPermission, err := acl.HasPermission(ctx, currentUser.GetRid(), config.RESOURCE_ACCOUNT, account.ResourceId, acl.READ)
		if err != nil {
			return nil, err
		}
		if !hasPermission {
			return nil, ErrAccountForbidden
		}
	}

	if err := repository.ResolveCredential(ctx.Request.Context(), &account); err != nil {
		return nil, err
	}
	return &account, nil
}
