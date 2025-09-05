package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// SystemConfigRepository interface for system config data access
type SystemConfigRepository interface {
	GetByKey(ctx context.Context, key string) (*model.SystemConfig, error)
	SetByKey(ctx context.Context, key, value string) error
}

// systemConfigRepository implements SystemConfigRepository
type systemConfigRepository struct{}

// NewSystemConfigRepository creates a new system config repository
func NewSystemConfigRepository() SystemConfigRepository {
	return &systemConfigRepository{}
}

// GetByKey gets system config by key
func (r *systemConfigRepository) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var config model.SystemConfig
	err := dbpkg.DB.WithContext(ctx).Where("config_key = ?", key).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// SetByKey sets system config by key
func (r *systemConfigRepository) SetByKey(ctx context.Context, key, value string) error {
	config := model.SystemConfig{
		Key:   key,
		Value: value,
	}
	
	return dbpkg.DB.WithContext(ctx).Where("config_key = ?", key).
		Assign(model.SystemConfig{Value: value}).
		FirstOrCreate(&config).Error
}