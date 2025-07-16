package service

import (
	"context"
	"time"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/cache"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
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

// MigrateConfigStructure migrates the config table structure from protocol-specific to unified permissions
func (s *ConfigService) MigrateConfigStructure(ctx context.Context) error {
	logger.L().Info("Starting config table structure migration")

	// Check if migration is needed by looking for old columns
	var columnExists bool
	if err := dbpkg.DB.Raw("SELECT COUNT(*) > 0 FROM information_schema.columns WHERE table_name = 'config' AND column_name = 'ssh_copy'").Scan(&columnExists).Error; err != nil {
		logger.L().Error("Failed to check for old columns", zap.Error(err))
		return err
	}

	if !columnExists {
		logger.L().Info("Config table already migrated, skipping")
		return nil
	}

	logger.L().Info("Config table migration needed, starting migration")

	// Migrate existing config data
	if err := dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Read existing config with old structure
		var oldConfig struct {
			ID        int  `gorm:"column:id"`
			Timeout   int  `gorm:"column:timeout"`
			SSHCopy   bool `gorm:"column:ssh_copy"`
			SSHPaste  bool `gorm:"column:ssh_paste"`
			RDPCopy   bool `gorm:"column:rdp_copy"`
			RDPPaste  bool `gorm:"column:rdp_paste"`
			VNCCopy   bool `gorm:"column:vnc_copy"`
			VNCPaste  bool `gorm:"column:vnc_paste"`
			CreatorId int  `gorm:"column:creator_id"`
			UpdaterId int  `gorm:"column:updater_id"`
		}

		if err := tx.Table("config").Where("deleted_at = 0").First(&oldConfig).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// No existing config, create default
				logger.L().Info("No existing config found, will create default after migration")
				return nil
			}
			return err
		}

		// Convert old config to new structure
		// Use logical OR for permissions - if any protocol allowed it, then allow it
		newConfig := &model.Config{
			Id:      oldConfig.ID,
			Timeout: oldConfig.Timeout,
			DefaultPermissions: model.DefaultPermissions{
				Connect:      true, // Always allow connect
				FileUpload:   true, // Default to allow file operations
				FileDownload: true,
				Copy:         oldConfig.SSHCopy || oldConfig.RDPCopy || oldConfig.VNCCopy,
				Paste:        oldConfig.SSHPaste || oldConfig.RDPPaste || oldConfig.VNCPaste,
				Share:        false, // Default to deny share for security
			},
			CreatorId: oldConfig.CreatorId,
			UpdaterId: oldConfig.UpdaterId,
		}

		// Store converted config data temporarily
		if err := tx.Exec("CREATE TEMPORARY TABLE temp_config AS SELECT * FROM config WHERE deleted_at = 0").Error; err != nil {
			return err
		}

		logger.L().Info("Migrated config permissions",
			zap.Bool("copy", newConfig.DefaultPermissions.Copy),
			zap.Bool("paste", newConfig.DefaultPermissions.Paste))

		return nil
	}); err != nil {
		logger.L().Error("Failed to migrate config data", zap.Error(err))
		return err
	}

	// Drop old columns and add new ones
	if err := dbpkg.DB.Transaction(func(tx *gorm.DB) error {
		// Add new columns
		alterSQL := `
			ALTER TABLE config 
			ADD COLUMN default_connect BOOLEAN DEFAULT TRUE,
			ADD COLUMN default_file_upload BOOLEAN DEFAULT TRUE,
			ADD COLUMN default_file_download BOOLEAN DEFAULT TRUE,
			ADD COLUMN default_copy BOOLEAN DEFAULT TRUE,
			ADD COLUMN default_paste BOOLEAN DEFAULT TRUE,
			ADD COLUMN default_share BOOLEAN DEFAULT FALSE
		`
		if err := tx.Exec(alterSQL).Error; err != nil {
			logger.L().Error("Failed to add new columns", zap.Error(err))
			return err
		}

		// Update data from temporary table if it exists
		updateSQL := `
			UPDATE config SET 
				default_connect = TRUE,
				default_file_upload = TRUE,
				default_file_download = TRUE,
				default_copy = COALESCE((SELECT (ssh_copy OR rdp_copy OR vnc_copy) FROM temp_config WHERE temp_config.id = config.id), TRUE),
				default_paste = COALESCE((SELECT (ssh_paste OR rdp_paste OR vnc_paste) FROM temp_config WHERE temp_config.id = config.id), TRUE),
				default_share = FALSE
			WHERE deleted_at = 0
		`
		if err := tx.Exec(updateSQL).Error; err != nil {
			logger.L().Error("Failed to update config data", zap.Error(err))
			return err
		}

		// Drop old columns
		dropSQL := `
			ALTER TABLE config 
			DROP COLUMN ssh_copy,
			DROP COLUMN ssh_paste,
			DROP COLUMN rdp_copy,
			DROP COLUMN rdp_paste,
			DROP COLUMN rdp_enable_drive,
			DROP COLUMN rdp_drive_path,
			DROP COLUMN rdp_create_drive_path,
			DROP COLUMN rdp_disable_upload,
			DROP COLUMN rdp_disable_download,
			DROP COLUMN vnc_copy,
			DROP COLUMN vnc_paste
		`
		if err := tx.Exec(dropSQL).Error; err != nil {
			logger.L().Error("Failed to drop old columns", zap.Error(err))
			return err
		}

		// Drop temporary table
		if err := tx.Exec("DROP TEMPORARY TABLE IF EXISTS temp_config").Error; err != nil {
			logger.L().Warn("Failed to drop temporary table", zap.Error(err))
		}

		return nil
	}); err != nil {
		return err
	}

	logger.L().Info("Config table structure migration completed successfully")
	return nil
}

// EnsureDefaultConfig ensures there's a default config in the database
func (s *ConfigService) EnsureDefaultConfig(ctx context.Context) error {
	cfg, err := s.GetConfig(ctx)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if cfg == nil {
		// Create default config
		defaultCfg := model.GetDefaultConfig()
		defaultCfg.CreatorId = 1 // System user
		defaultCfg.UpdaterId = 1

		if err := s.SaveConfig(ctx, defaultCfg); err != nil {
			return err
		}

		logger.L().Info("Created default config")
	}

	return nil
}
