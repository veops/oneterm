package repository

import (
	"context"
	"time"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// StatRepository defines the interface for stat repository
type StatRepository interface {
	GetStatNodeAssetTypes(ctx context.Context) ([]*model.StatAssetType, error)
	GetStatCount(ctx context.Context) (*model.StatCount, error)
	GetStatAccount(ctx context.Context, start, end time.Time) ([]*model.StatAccount, error)
	GetStatAsset(ctx context.Context, start, end time.Time, dateFmt string) ([]*model.StatAsset, error)
	GetStatCountOfUser(ctx context.Context, uid int, assetIds []int) (*model.StatCountOfUser, error)
	GetStatRankOfUser(ctx context.Context, limit int) ([]*model.StatRankOfUser, error)
}

type statRepository struct{}

// NewStatRepository creates a new stat repository
func NewStatRepository() StatRepository {
	return &statRepository{}
}

// GetStatNodeAssetTypes gets node asset types
func (r *statRepository) GetStatNodeAssetTypes(ctx context.Context) ([]*model.StatAssetType, error) {
	stat := make([]*model.StatAssetType, 0)
	if err := dbpkg.DB.
		Model(stat).
		Where("parent_id = 0").
		Find(&stat).
		Error; err != nil {
		return nil, err
	}
	return stat, nil
}

// GetStatCount gets stats count
func (r *statRepository) GetStatCount(ctx context.Context) (*model.StatCount, error) {
	stat := &model.StatCount{}

	// Get online stats
	if err := dbpkg.DB.
		Model(model.DefaultSession).
		Select("COUNT(DISTINCT asset_id, account_id) as connect, COUNT(DISTINCT uid) as user, COUNT(DISTINCT gateway_id) as gateway, COUNT(*) as session").
		Where("status = 1").
		First(&stat).
		Error; err != nil {
		return nil, err
	}

	// Get asset count
	if err := dbpkg.DB.Model(model.DefaultAsset).Count(&stat.TotalAsset).Error; err != nil {
		return nil, err
	}

	// Get connectable asset count
	if err := dbpkg.DB.Model(model.DefaultAsset).Where("connectable = 1").Count(&stat.Asset).Error; err != nil {
		return nil, err
	}

	// Get gateway count
	if err := dbpkg.DB.Model(model.DefaultGateway).Count(&stat.TotalGateway).Error; err != nil {
		return nil, err
	}

	return stat, nil
}

// GetStatAccount gets account stats
func (r *statRepository) GetStatAccount(ctx context.Context, start, end time.Time) ([]*model.StatAccount, error) {
	stat := make([]*model.StatAccount, 0)

	err := dbpkg.DB.
		Model(&model.Account{}).
		Select("account.name, COUNT(*) AS count").
		Joins("LEFT JOIN session ON account.id = session.account_id").
		Group("account.id").
		Order("count DESC").
		Limit(10).
		Where("session.created_at >= ? AND session.created_at <= ?", start, end).
		Find(&stat).
		Error

	return stat, err
}

// GetStatAsset gets asset stats
func (r *statRepository) GetStatAsset(ctx context.Context, start, end time.Time, dateFmt string) ([]*model.StatAsset, error) {
	stat := make([]*model.StatAsset, 0)

	err := dbpkg.DB.
		Model(model.DefaultSession).
		Select("COUNT(DISTINCT asset_id, uid) AS connect, COUNT(*) AS session, COUNT(DISTINCT asset_id) AS asset, COUNT(DISTINCT uid) AS user, DATE_FORMAT(created_at, ?) AS time", dateFmt).
		Where("session.created_at >= ? AND session.created_at <= ?", start, end).
		Group("time").
		Find(&stat).
		Error

	return stat, err
}

// GetStatCountOfUser gets stats count for a specific user
func (r *statRepository) GetStatCountOfUser(ctx context.Context, uid int, assetIds []int) (*model.StatCountOfUser, error) {
	stat := &model.StatCountOfUser{}

	// Get user's session stats
	if err := dbpkg.DB.
		Model(model.DefaultSession).
		Select("COUNT(DISTINCT asset_id, account_id) as connect, COUNT(DISTINCT asset_id) as asset, COUNT(*) as session").
		Where("status = 1").
		Where("uid = ?", uid).
		First(&stat).
		Error; err != nil {
		return nil, err
	}

	// Set total asset count based on user's accessible assets
	stat.TotalAsset = int64(len(assetIds))

	return stat, nil
}

// GetStatRankOfUser gets user rank stats
func (r *statRepository) GetStatRankOfUser(ctx context.Context, limit int) ([]*model.StatRankOfUser, error) {
	stat := make([]*model.StatRankOfUser, 0)

	err := dbpkg.DB.
		Model(model.DefaultSession).
		Select("uid, COUNT(*) AS count, MAX(created_at) AS last_time").
		Group("uid").
		Order("count DESC").
		Limit(limit).
		Find(&stat).
		Error

	return stat, err
}
