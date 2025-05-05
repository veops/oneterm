package repository

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// ShareRepository defines the interface for share repository
type ShareRepository interface {
	GetShareByID(ctx context.Context, id int) (*model.Share, error)
	GetShareByUUID(ctx context.Context, uuid string) (*model.Share, error)
	CreateShares(ctx context.Context, shares []*model.Share) error
	BuildQuery(ctx *gin.Context, assetIds, accountIds []int) (*gorm.DB, error)
	DecrementShareTimes(ctx context.Context, uuid string) (int64, error)
}

type shareRepository struct{}

// NewShareRepository creates a new share repository
func NewShareRepository() ShareRepository {
	return &shareRepository{}
}

// GetShareByID retrieves a share by ID
func (r *shareRepository) GetShareByID(ctx context.Context, id int) (*model.Share, error) {
	share := &model.Share{}
	if err := dbpkg.DB.Model(share).Where("id=?", id).First(share).Error; err != nil {
		return nil, err
	}
	return share, nil
}

// GetShareByUUID retrieves a share by UUID
func (r *shareRepository) GetShareByUUID(ctx context.Context, uuid string) (*model.Share, error) {
	share := &model.Share{}
	if err := dbpkg.DB.Where("uuid=?", uuid).First(share).Error; err != nil {
		return nil, err
	}
	return share, nil
}

// CreateShares creates new shares
func (r *shareRepository) CreateShares(ctx context.Context, shares []*model.Share) error {
	return dbpkg.DB.Create(&shares).Error
}

// BuildQuery constructs a query for shares with filters
func (r *shareRepository) BuildQuery(ctx *gin.Context, assetIds, accountIds []int) (*gorm.DB, error) {
	db := dbpkg.DB.Model(&model.Share{})

	// Apply text search
	if q, ok := ctx.GetQuery("search"); ok && q != "" {
		db = db.Where("name LIKE ? OR ip LIKE ?", "%"+q+"%", "%"+q+"%")
	}

	// Apply date range filters
	if start, ok := ctx.GetQuery("start"); ok {
		t, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return nil, err
		}
		db = db.Where("created_at >= ?", t)
	}

	if end, ok := ctx.GetQuery("end"); ok {
		t, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return nil, err
		}
		db = db.Where("created_at <= ?", t)
	}

	// Apply exact match filters
	for _, field := range []string{"asset_id", "account_id"} {
		if q, ok := ctx.GetQuery(field); ok && q != "" {
			db = db.Where(field+" = ?", q)
		}
	}

	// Apply asset and account id filters if provided
	if len(assetIds) > 0 || len(accountIds) > 0 {
		db = db.Where("asset_id IN (?) OR account_id IN (?)", assetIds, accountIds)
	}

	return db, nil
}

// DecrementShareTimes decrements the times field for a share
func (r *shareRepository) DecrementShareTimes(ctx context.Context, uuid string) (int64, error) {
	db := dbpkg.DB.Model(&model.Share{}).Where("uuid=? AND times>0", uuid).Update("times", gorm.Expr("times-?", 1))
	return db.RowsAffected, db.Error
}
