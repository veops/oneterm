package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// ConfigRepository defines the interface for config repository
type ConfigRepository interface {
	GetConfig(ctx context.Context) (*model.Config, error)
	SaveConfig(ctx context.Context, cfg *model.Config) error
}

type configRepository struct{}

// NewConfigRepository creates a new config repository
func NewConfigRepository() ConfigRepository {
	return &configRepository{}
}

// GetConfig retrieves the current configuration
func (r *configRepository) GetConfig(ctx context.Context) (*model.Config, error) {
	cfg := &model.Config{}
	if err := dbpkg.DB.Model(cfg).First(cfg).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig saves a configuration
func (r *configRepository) SaveConfig(ctx context.Context, cfg *model.Config) error {
	return dbpkg.DB.Create(cfg).Error
}
