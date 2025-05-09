package repository

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// PublicKeyRepository defines the interface for public key repository
type PublicKeyRepository interface {
	GetPublicKey(ctx context.Context, id int) (*model.PublicKey, error)
	BuildQuery(ctx *gin.Context, uid int) *gorm.DB
}

type publicKeyRepository struct{}

// NewPublicKeyRepository creates a new public key repository
func NewPublicKeyRepository() PublicKeyRepository {
	return &publicKeyRepository{}
}

// GetPublicKey retrieves a public key by ID
func (r *publicKeyRepository) GetPublicKey(ctx context.Context, id int) (*model.PublicKey, error) {
	publicKey := &model.PublicKey{}
	if err := dbpkg.DB.Where("id = ?", id).First(publicKey).Error; err != nil {
		return nil, err
	}
	return publicKey, nil
}

// BuildQuery constructs a query for public keys with filters
func (r *publicKeyRepository) BuildQuery(ctx *gin.Context, uid int) *gorm.DB {
	db := dbpkg.DB.Model(&model.PublicKey{})

	// Filter by search terms
	if q, ok := ctx.GetQuery("search"); ok && q != "" {
		db = db.Where("name LIKE ? OR mac LIKE ?", "%"+q+"%", "%"+q+"%")
	}

	// Filter by ID
	if q, ok := ctx.GetQuery("id"); ok && q != "" {
		db = db.Where("id = ?", q)
	}

	// Filter by name
	if q, ok := ctx.GetQuery("name"); ok && q != "" {
		db = db.Where("name LIKE ?", "%"+q+"%")
	}

	// Filter by user ID
	db = db.Where("uid = ?", uid)

	return db
}
