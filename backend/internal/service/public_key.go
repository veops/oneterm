package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/utils"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// PublicKeyService handles public key business logic
type PublicKeyService struct {
	repo repository.PublicKeyRepository
}

// NewPublicKeyService creates a new public key service
func NewPublicKeyService() *PublicKeyService {
	return &PublicKeyService{
		repo: repository.NewPublicKeyRepository(),
	}
}

// ValidatePublicKey validates the given public key format
func (s *PublicKeyService) ValidatePublicKey(publicKey *model.PublicKey) error {
	_, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey.Pk))
	if err != nil {
		return err
	}

	// Clean the key
	publicKey.Pk = strings.TrimSpace(strings.TrimSuffix(publicKey.Pk, comment))
	return nil
}

// EncryptPublicKey encrypts the public key
func (s *PublicKeyService) EncryptPublicKey(publicKey *model.PublicKey) {
	publicKey.Pk = utils.EncryptAES(publicKey.Pk)
}

// DecryptPublicKeys decrypts public keys
func (s *PublicKeyService) DecryptPublicKeys(publicKeys []*model.PublicKey) {
	for _, pk := range publicKeys {
		pk.Pk = utils.DecryptAES(pk.Pk)
	}
}

// SetUserInfo sets user information for the public key
func (s *PublicKeyService) SetUserInfo(ctx context.Context, publicKey *model.PublicKey) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	publicKey.Uid = currentUser.GetUid()
	publicKey.UserName = currentUser.GetUserName()
}

// BuildQuery constructs a query for public keys
func (s *PublicKeyService) BuildQuery(ctx *gin.Context) *gorm.DB {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	return s.repo.BuildQuery(ctx, currentUser.Uid)
}

// GetPublicKey retrieves a public key by ID
func (s *PublicKeyService) GetPublicKey(ctx context.Context, id int) (*model.PublicKey, error) {
	return s.repo.GetPublicKey(ctx, id)
}
