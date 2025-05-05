package service

import (
	"context"
	"time"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/cache"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// ConfigService handles configuration business logic
type ConfigService struct {
	repo repository.ConfigRepository
}

// NewConfigService creates a new config service
func NewConfigService() *ConfigService {
	return &ConfigService{
		repo: repository.NewConfigRepository(),
	}
}

// SaveConfig saves a configuration
func (s *ConfigService) SaveConfig(ctx context.Context, cfg *model.Config) error {
	cfg.Id = 0 // Ensure we're creating a new config

	if err := dbpkg.DB.Model(cfg).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deleted_at = 0").Delete(&model.Config{}).Error; err != nil {
			return err
		}
		return tx.Create(cfg).Error
	}); err != nil {
		return err
	}

	// Update global config and cache
	model.GlobalConfig.Store(cfg)
	cache.SetEx(ctx, "config", cfg, time.Hour)

	return nil
}

// GetConfig retrieves the current configuration
func (s *ConfigService) GetConfig(ctx context.Context) (*model.Config, error) {
	return s.repo.GetConfig(ctx)
}
