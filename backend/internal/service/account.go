package service

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/db"
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
	purpose := ctx.DefaultQuery("purpose", "access")
	if purpose != "access" && purpose != "manage" && purpose != "grant" {
		return nil, ErrPAMInput
	}
	// Start with base query
	db := s.repo.BuildQuery(ctx)

	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if currentUser == nil || currentUser.Uid <= 0 {
		return nil, ErrPAMDenied
	}

	// Administrators have access to all accounts
	if acl.IsAdmin(currentUser) {
		return db, nil
	}
	var managementIDs []int
	if purpose != "access" {
		var err error
		managementIDs, err = accountManagementResources(ctx, purpose == "grant")
		if err != nil {
			return nil, err
		}
		if purpose == "grant" {
			return db.Where("resource_id IN ?", managementIDs), nil
		}
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
		db = db.Where("resource_id IN ?", managementIDs)
	} else {
		db = db.Where("(id IN ? OR resource_id IN ?)", accountIds, managementIDs)
	}

	return db, nil
}

// GetAccountCredentials preserves basic reads and enforces configured credential policies.
func (s *AccountService) GetAccountCredentials(ctx *gin.Context, accountId int) (*model.Account, error) {
	var account model.Account
	if err := repository.NewBaseRepository().GetById(ctx, accountId, &account); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	// Preserve the basic reader; managed accounts and approval policies use protected retrieval.
	if !account.Managed {
		policy, err := repository.PasswordViewPolicy(db.GetDB().WithContext(ctx.Request.Context()))
		if err != nil {
			return nil, ErrPAMUnavailable
		}
		if !requiresPAMApproval(policy, acl.Retrieve) && len(policy.SourceCIDRs) == 0 {
			operator, err := acl.GetSessionFromCtx(ctx)
			if err != nil || operator == nil || operator.Uid <= 0 {
				return nil, ErrPAMDenied
			}
			if !acl.IsAdmin(operator) {
				allowed, err := pamResourcePermission(ctx.Request.Context(), operator, &account, acl.READ, false)
				if err != nil {
					return nil, ErrPAMUnavailable
				}
				if !allowed {
					return nil, ErrPAMDenied
				}
			}
			if err := repository.ResolveCredential(ctx.Request.Context(), &account); err != nil {
				return nil, err
			}
			return &account, nil
		}
	}
	credential, err := retrievePasswordView(ctx, model.PAMCredentialTarget{Kind: model.PAMOwnerAccount, ID: accountId}, false)
	if err != nil {
		return nil, err
	}
	account.SetCredentialValues(credential.Password, credential.PK, credential.Phrase)
	return &account, nil
}
