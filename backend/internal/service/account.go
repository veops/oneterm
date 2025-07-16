package service

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/utils"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
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

// ValidatePublicKey validates the given public key
func (s *AccountService) ValidatePublicKey(account *model.Account) error {
	if account.AccountType != model.AUTHMETHOD_PUBLICKEY {
		return nil
	}

	var err error
	if account.Phrase == "" {
		_, err = ssh.ParsePrivateKey([]byte(account.Pk))
	} else {
		_, err = ssh.ParsePrivateKeyWithPassphrase([]byte(account.Pk), []byte(account.Phrase))
	}
	return err
}

// EncryptSensitiveData encrypts sensitive account data
func (s *AccountService) EncryptSensitiveData(account *model.Account) {
	account.Password = utils.EncryptAES(account.Password)
	account.Pk = utils.EncryptAES(account.Pk)
	account.Phrase = utils.EncryptAES(account.Phrase)
}

// DecryptSensitiveData decrypts sensitive account data
func (s *AccountService) DecryptSensitiveData(accounts []*model.Account) {
	for _, a := range accounts {
		a.Password = utils.DecryptAES(a.Password)
		a.Pk = utils.DecryptAES(a.Pk)
		a.Phrase = utils.DecryptAES(a.Phrase)
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

// FilterByAssetIds filters accounts by related asset IDs
func (s *AccountService) FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB {
	return s.repo.FilterByAssetIds(db, assetIds)
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

	// Apply V2 authorization filter: get authorized asset IDs using V2 system
	authV2Service := NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	if err != nil {
		return nil, err
	}

	// Use the same filtering logic as before, but with V2 authorized assets
	// This maintains the original logic: find accounts that can access the authorized assets
	return s.FilterByAssetIds(db, assetIds), nil
}
