package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"go.uber.org/zap"
)

// StorageCleanerService handles storage cleanup and archival tasks
type StorageCleanerService struct {
	storageService StorageService
	ticker         *time.Ticker
	stopChan       chan struct{}
}

// NewStorageCleanerService creates a new storage cleaner service
func NewStorageCleanerService(storageService StorageService) *StorageCleanerService {
	return &StorageCleanerService{
		storageService: storageService,
		stopChan:       make(chan struct{}),
	}
}

// Start starts the storage cleaner with daily checks at 2 AM
func (s *StorageCleanerService) Start() {
	// Calculate time until next 2 AM
	now := time.Now()
	next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if now.After(next2AM) {
		next2AM = next2AM.Add(24 * time.Hour)
	}

	// Wait until 2 AM, then run every 24 hours
	timer := time.NewTimer(time.Until(next2AM))

	go func() {
		<-timer.C
		s.runCleanup()

		// Now run every 24 hours
		s.ticker = time.NewTicker(24 * time.Hour)
		for {
			select {
			case <-s.ticker.C:
				s.runCleanup()
			case <-s.stopChan:
				return
			}
		}
	}()

	logger.L().Info("Storage cleaner service started",
		zap.Time("next_run", next2AM))
}

// Stop stops the storage cleaner service
func (s *StorageCleanerService) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopChan)
	logger.L().Info("Storage cleaner service stopped")
}

// runCleanup performs the actual cleanup and archival
func (s *StorageCleanerService) runCleanup() {
	logger.L().Info("Starting storage cleanup and archival")

	ctx := context.Background()
	configs, err := s.storageService.GetStorageConfigs(ctx)
	if err != nil {
		logger.L().Error("Failed to get storage configs for cleanup", zap.Error(err))
		return
	}

	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		provider, err := s.storageService.CreateProvider(config)
		if err != nil {
			logger.L().Error("Failed to create provider for cleanup",
				zap.String("storage", config.Name), zap.Error(err))
			continue
		}

		s.cleanupStorage(config, provider)
	}

	logger.L().Info("Storage cleanup and archival completed")
}

// cleanupStorage performs cleanup for a specific storage provider
func (s *StorageCleanerService) cleanupStorage(config *model.StorageConfig, provider storage.Provider) {
	// Only handle local storage for now
	if config.Type != model.StorageTypeLocal {
		return
	}

	basePath := config.Config["base_path"]
	if basePath == "" {
		return
	}

	// Parse retention config
	retentionDays := 30 // default
	archiveDays := 7    // default
	cleanupEnabled := true
	archiveEnabled := true

	if val, exists := config.Config["retention_days"]; exists {
		if days, err := parseIntConfig(val); err == nil {
			retentionDays = days
		}
	}
	if val, exists := config.Config["archive_days"]; exists {
		if days, err := parseIntConfig(val); err == nil {
			archiveDays = days
		}
	}
	if val, exists := config.Config["cleanup_enabled"]; exists {
		cleanupEnabled = val == "true"
	}
	if val, exists := config.Config["archive_enabled"]; exists {
		archiveEnabled = val == "true"
	}

	logger.L().Info("Processing storage cleanup",
		zap.String("storage", config.Name),
		zap.String("base_path", basePath),
		zap.Int("retention_days", retentionDays),
		zap.Int("archive_days", archiveDays),
		zap.Bool("cleanup_enabled", cleanupEnabled),
		zap.Bool("archive_enabled", archiveEnabled))

	// Get all date directories
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logger.L().Error("Failed to read base directory",
			zap.String("path", basePath), zap.Error(err))
		return
	}

	now := time.Now()
	retentionCutoff := now.AddDate(0, 0, -retentionDays)
	archiveCutoff := now.AddDate(0, 0, -archiveDays)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name looks like a date (YYYY-MM-DD)
		dirName := entry.Name()
		if len(dirName) != 10 || dirName[4] != '-' || dirName[7] != '-' {
			continue
		}

		dirDate, err := time.Parse("2006-01-02", dirName)
		if err != nil {
			continue
		}

		dirPath := filepath.Join(basePath, dirName)

		// Delete directories older than retention period
		if cleanupEnabled && dirDate.Before(retentionCutoff) {
			logger.L().Info("Deleting expired directory",
				zap.String("path", dirPath),
				zap.Time("date", dirDate))

			if err := os.RemoveAll(dirPath); err != nil {
				logger.L().Error("Failed to delete directory",
					zap.String("path", dirPath), zap.Error(err))
			} else {
				logger.L().Info("Successfully deleted expired directory",
					zap.String("path", dirPath))
			}
			continue
		}

		// Archive directories older than archive period
		if archiveEnabled && dirDate.Before(archiveCutoff) {
			s.archiveDirectory(basePath, dirPath, dirDate)
		}
	}
}

// archiveDirectory archives a directory to archived folder
func (s *StorageCleanerService) archiveDirectory(basePath, dirPath string, dirDate time.Time) {
	archiveDir := filepath.Join(basePath, "archived")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		logger.L().Error("Failed to create archive directory",
			zap.String("path", archiveDir), zap.Error(err))
		return
	}

	archivedDirName := fmt.Sprintf("%s_archived", filepath.Base(dirPath))
	archivedDirPath := filepath.Join(archiveDir, archivedDirName)

	// Check if archive already exists
	if _, err := os.Stat(archivedDirPath); err == nil {
		logger.L().Info("Archive already exists, skipping",
			zap.String("archive", archivedDirPath))
		return
	}

	logger.L().Info("Archiving directory",
		zap.String("source", dirPath),
		zap.String("archive", archivedDirPath))

	// Move directory to archived folder
	if err := os.Rename(dirPath, archivedDirPath); err != nil {
		logger.L().Error("Failed to archive directory",
			zap.String("source", dirPath),
			zap.String("dest", archivedDirPath),
			zap.Error(err))
	} else {
		logger.L().Info("Successfully archived directory",
			zap.String("source", dirPath),
			zap.String("dest", archivedDirPath))
	}
}

// parseIntConfig parses integer from string config
func parseIntConfig(val string) (int, error) {
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	return result, err
}

// Global storage cleaner instance
var DefaultStorageCleanerService *StorageCleanerService

// InitStorageCleanerService initializes the global storage cleaner service
func InitStorageCleanerService() {
	if DefaultStorageService == nil {
		logger.L().Warn("Storage service not initialized, skipping cleaner initialization")
		return
	}

	DefaultStorageCleanerService = NewStorageCleanerService(DefaultStorageService)
	DefaultStorageCleanerService.Start()

	logger.L().Info("Storage cleaner service initialized")
}
