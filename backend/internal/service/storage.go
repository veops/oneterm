package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
	"github.com/veops/oneterm/pkg/storage/providers"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// StorageService defines the interface for storage operations
type StorageService interface {
	BaseService

	// Configuration Management using database
	GetStorageConfigs(ctx context.Context) ([]*model.StorageConfig, error)
	GetStorageConfig(ctx context.Context, name string) (*model.StorageConfig, error)
	CreateStorageConfig(ctx context.Context, config *model.StorageConfig) error
	UpdateStorageConfig(ctx context.Context, config *model.StorageConfig) error
	DeleteStorageConfig(ctx context.Context, name string) error

	// File Operations combining storage backend and database metadata
	UploadFile(ctx context.Context, key string, reader io.Reader, size int64, metadata *model.FileMetadata) error
	DownloadFile(ctx context.Context, key string) (io.ReadCloser, *model.FileMetadata, error)
	DeleteFile(ctx context.Context, key string) error
	FileExists(ctx context.Context, key string) (bool, error)
	ListFiles(ctx context.Context, prefix string, limit, offset int) ([]*model.FileMetadata, int64, error)

	// Business-specific operations for external interface
	SaveSessionReplay(ctx context.Context, sessionId string, reader io.Reader, size int64) error
	GetSessionReplay(ctx context.Context, sessionId string) (io.ReadCloser, error)
	DeleteSessionReplay(ctx context.Context, sessionId string) error

	SaveRDPFile(ctx context.Context, assetId int, remotePath string, reader io.Reader, size int64) error
	GetRDPFile(ctx context.Context, assetId int, remotePath string) (io.ReadCloser, error)
	DeleteRDPFile(ctx context.Context, assetId int, remotePath string) error

	// Provider management
	GetPrimaryProvider() (storage.Provider, error)
	HealthCheck(ctx context.Context) map[string]error
	CreateProvider(config *model.StorageConfig) (storage.Provider, error)

	// New method for building queries
	BuildQuery(ctx *gin.Context) *gorm.DB

	// GetAvailableProvider returns an available storage provider with fallback logic
	// Priority: Primary storage first, then by priority (lower number = higher priority)
	GetAvailableProvider(ctx context.Context) (storage.Provider, error)
}

// storageService implements StorageService
type storageService struct {
	BaseService
	storageRepo repository.StorageRepository
	providers   map[string]storage.Provider
	primary     string
}

// NewStorageService creates a new storage service
func NewStorageService() StorageService {
	return &storageService{
		BaseService: NewBaseService(),
		storageRepo: repository.NewStorageRepository(),
		providers:   make(map[string]storage.Provider),
	}
}

// Configuration Management via repository to operate database

func (s *storageService) BuildQuery(ctx *gin.Context) *gorm.DB {
	return s.storageRepo.BuildQuery(ctx)
}

func (s *storageService) GetStorageConfigs(ctx context.Context) ([]*model.StorageConfig, error) {
	return s.storageRepo.GetStorageConfigs(ctx)
}

func (s *storageService) GetStorageConfig(ctx context.Context, name string) (*model.StorageConfig, error) {
	return s.storageRepo.GetStorageConfigByName(ctx, name)
}

func (s *storageService) CreateStorageConfig(ctx context.Context, config *model.StorageConfig) error {
	// Validate configuration
	if err := s.validateConfig(config); err != nil {
		return err
	}

	// Create in database via repository
	if err := s.storageRepo.CreateStorageConfig(ctx, config); err != nil {
		return fmt.Errorf("failed to create storage config: %w", err)
	}

	// Initialize storage provider
	provider, err := s.CreateProvider(config)
	if err != nil {
		logger.L().Warn("Failed to initialize storage provider",
			zap.String("name", config.Name),
			zap.Error(err))
	} else {
		s.providers[config.Name] = provider
		if config.IsPrimary {
			s.primary = config.Name
		}
	}

	return nil
}

func (s *storageService) UpdateStorageConfig(ctx context.Context, config *model.StorageConfig) error {
	if err := s.validateConfig(config); err != nil {
		return err
	}

	return s.storageRepo.UpdateStorageConfig(ctx, config)
}

func (s *storageService) DeleteStorageConfig(ctx context.Context, name string) error {
	// Remove from memory
	delete(s.providers, name)
	if s.primary == name {
		s.primary = ""
	}

	// Delete from database
	return s.storageRepo.DeleteStorageConfig(ctx, name)
}

// File Operations combining storage provider and database metadata

func (s *storageService) UploadFile(ctx context.Context, key string, reader io.Reader, size int64, metadata *model.FileMetadata) error {
	provider, err := s.GetAvailableProvider(ctx)
	if err != nil {
		return fmt.Errorf("no available storage provider: %w", err)
	}

	// Upload to storage backend
	if err := provider.Upload(ctx, key, reader, size); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Save metadata to database if provided
	if metadata != nil {
		metadata.StorageKey = key
		metadata.FileSize = size
		metadata.StorageType = model.StorageType(provider.Type())

		if err := s.storageRepo.CreateFileMetadata(ctx, metadata); err != nil {
			logger.L().Warn("Failed to save file metadata",
				zap.String("key", key),
				zap.Error(err))
		}
	}

	return nil
}

func (s *storageService) DownloadFile(ctx context.Context, key string) (io.ReadCloser, *model.FileMetadata, error) {
	provider, err := s.GetAvailableProvider(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("no available storage provider: %w", err)
	}

	// Download from storage backend
	reader, err := provider.Download(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download file: %w", err)
	}

	// Get metadata from database
	metadata, err := s.storageRepo.GetFileMetadata(ctx, key)
	if err != nil {
		logger.L().Warn("Failed to get file metadata",
			zap.String("key", key),
			zap.Error(err))
	}

	return reader, metadata, nil
}

func (s *storageService) DeleteFile(ctx context.Context, key string) error {
	provider, err := s.GetAvailableProvider(ctx)
	if err != nil {
		return fmt.Errorf("no available storage provider: %w", err)
	}

	// Delete from storage backend
	if err := provider.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata from database
	if err := s.storageRepo.DeleteFileMetadata(ctx, key); err != nil {
		logger.L().Warn("Failed to delete file metadata",
			zap.String("key", key),
			zap.Error(err))
	}

	return nil
}

func (s *storageService) FileExists(ctx context.Context, key string) (bool, error) {
	provider, err := s.GetPrimaryProvider()
	if err != nil {
		return false, fmt.Errorf("no primary storage provider: %w", err)
	}

	return provider.Exists(ctx, key)
}

func (s *storageService) ListFiles(ctx context.Context, prefix string, limit, offset int) ([]*model.FileMetadata, int64, error) {
	return s.storageRepo.ListFileMetadata(ctx, prefix, limit, offset)
}

// Business-specific operations

func (s *storageService) SaveSessionReplay(ctx context.Context, sessionId string, reader io.Reader, size int64) error {
	key := fmt.Sprintf("%s.cast", sessionId)

	metadata := &model.FileMetadata{
		FileName:  fmt.Sprintf("%s.cast", sessionId),
		Category:  "replay",
		SessionId: sessionId,
		MimeType:  "application/octet-stream",
	}

	logger.L().Info("SaveReplay called", zap.String("session_id", sessionId))
	return s.UploadFile(ctx, key, reader, size, metadata)
}

func (s *storageService) GetSessionReplay(ctx context.Context, sessionId string) (io.ReadCloser, error) {
	key := fmt.Sprintf("%s.cast", sessionId)
	reader, _, err := s.DownloadFile(ctx, key)
	return reader, err
}

func (s *storageService) DeleteSessionReplay(ctx context.Context, sessionId string) error {
	key := fmt.Sprintf("%s.cast", sessionId)
	return s.DeleteFile(ctx, key)
}

func (s *storageService) SaveRDPFile(ctx context.Context, assetId int, remotePath string, reader io.Reader, size int64) error {
	// Normalize path format
	normalizedPath := filepath.ToSlash(strings.TrimPrefix(remotePath, "/"))
	key := fmt.Sprintf("rdp/asset_%d/%s", assetId, normalizedPath)

	metadata := &model.FileMetadata{
		FileName: filepath.Base(remotePath),
		Category: "rdp_file",
		AssetId:  assetId,
	}

	return s.UploadFile(ctx, key, reader, size, metadata)
}

func (s *storageService) GetRDPFile(ctx context.Context, assetId int, remotePath string) (io.ReadCloser, error) {
	normalizedPath := filepath.ToSlash(strings.TrimPrefix(remotePath, "/"))
	key := fmt.Sprintf("rdp/asset_%d/%s", assetId, normalizedPath)
	reader, _, err := s.DownloadFile(ctx, key)
	return reader, err
}

func (s *storageService) DeleteRDPFile(ctx context.Context, assetId int, remotePath string) error {
	normalizedPath := filepath.ToSlash(strings.TrimPrefix(remotePath, "/"))
	key := fmt.Sprintf("rdp/asset_%d/%s", assetId, normalizedPath)
	return s.DeleteFile(ctx, key)
}

// Provider management

func (s *storageService) GetPrimaryProvider() (storage.Provider, error) {
	if s.primary == "" {
		return nil, fmt.Errorf("no primary storage provider configured")
	}

	provider, exists := s.providers[s.primary]
	if !exists {
		return nil, fmt.Errorf("primary storage provider not found: %s", s.primary)
	}

	return provider, nil
}

func (s *storageService) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)

	for name, provider := range s.providers {
		err := provider.HealthCheck(ctx)
		results[name] = err
	}

	return results
}

// Helper methods

func (s *storageService) validateConfig(config *model.StorageConfig) error {
	if config.Name == "" {
		return fmt.Errorf("storage name is required")
	}

	if config.Type == "" {
		return fmt.Errorf("storage type is required")
	}

	return nil
}

func (s *storageService) CreateProvider(config *model.StorageConfig) (storage.Provider, error) {
	switch config.Type {
	case model.StorageTypeLocal:
		localConfig := providers.LocalConfig{
			BasePath: config.Config["base_path"],
		}

		// Parse path strategy from config
		if strategyStr, exists := config.Config["path_strategy"]; exists {
			localConfig.PathStrategy = storage.PathStrategy(strategyStr)
		} else {
			localConfig.PathStrategy = storage.DateHierarchyStrategy // Default to date hierarchy
		}

		// Parse retention configuration
		retentionConfig := storage.DefaultRetentionConfig()
		if retentionDaysStr, exists := config.Config["retention_days"]; exists {
			if days, err := strconv.Atoi(retentionDaysStr); err == nil {
				retentionConfig.RetentionDays = days
			}
		}
		if archiveDaysStr, exists := config.Config["archive_days"]; exists {
			if days, err := strconv.Atoi(archiveDaysStr); err == nil {
				retentionConfig.ArchiveDays = days
			}
		}
		if cleanupStr, exists := config.Config["cleanup_enabled"]; exists {
			retentionConfig.CleanupEnabled = cleanupStr == "true"
		}
		if archiveStr, exists := config.Config["archive_enabled"]; exists {
			retentionConfig.ArchiveEnabled = archiveStr == "true"
		}
		localConfig.RetentionConfig = retentionConfig

		return providers.NewLocal(localConfig)
	case model.StorageTypeMinio:
		minioConfig, err := providers.ParseMinioConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Minio config: %w", err)
		}
		return providers.NewMinio(minioConfig)
	case model.StorageTypeS3:
		// TODO: implement S3 provider with path strategy support
		return nil, fmt.Errorf("S3 provider not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// GetAvailableProvider returns an available storage provider with fallback logic
// Priority: Primary storage first, then by priority (lower number = higher priority)
func (s *storageService) GetAvailableProvider(ctx context.Context) (storage.Provider, error) {
	// 1. Try primary storage first
	if s.primary != "" {
		if provider, exists := s.providers[s.primary]; exists {
			if healthErr := provider.HealthCheck(ctx); healthErr == nil {
				return provider, nil
			} else {
				logger.L().Warn("Primary storage provider health check failed, trying fallback",
					zap.String("primary", s.primary),
					zap.Error(healthErr))
			}
		}
	}

	// 2. Get all enabled storage configs sorted by priority
	configs, err := s.GetStorageConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage configs: %w", err)
	}

	// Filter enabled configs and sort by priority (lower number = higher priority)
	var enabledConfigs []*model.StorageConfig
	for _, config := range configs {
		if config.Enabled {
			enabledConfigs = append(enabledConfigs, config)
		}
	}

	// Sort by priority (ascending order - lower number = higher priority)
	sort.Slice(enabledConfigs, func(i, j int) bool {
		return enabledConfigs[i].Priority < enabledConfigs[j].Priority
	})

	// 3. Try each provider by priority order
	for _, config := range enabledConfigs {
		if provider, exists := s.providers[config.Name]; exists {
			if healthErr := provider.HealthCheck(ctx); healthErr == nil {
				logger.L().Info("Using fallback storage provider",
					zap.String("name", config.Name),
					zap.Int("priority", config.Priority))
				return provider, nil
			} else {
				logger.L().Warn("Storage provider health check failed",
					zap.String("name", config.Name),
					zap.Error(healthErr))
			}
		}
	}

	return nil, fmt.Errorf("no available storage provider found")
}

// Global storage service instance
var DefaultStorageService StorageService

// InitStorageService initializes the global storage service with database configurations
func InitStorageService() {
	if DefaultStorageService == nil {
		DefaultStorageService = NewStorageService()
	}

	ctx := context.Background()
	storageImpl := DefaultStorageService.(*storageService)

	// 1. Load or create storage configurations
	configs, err := loadOrCreateStorageConfigs(ctx, storageImpl)
	if err != nil {
		logger.L().Error("Failed to initialize storage configurations", zap.Error(err))
		return
	}

	// 2. Validate configuration status
	validateStorageConfigs(configs)

	// 3. Initialize storage providers
	successCount := initializeStorageProviders(ctx, storageImpl, configs)

	// 4. Verify primary provider
	if err := verifyPrimaryProvider(ctx, storageImpl); err != nil {
		logger.L().Error("Primary storage provider verification failed", zap.Error(err))
		return
	}

	// 5. Initialize session replay adapter
	provider, err := storageImpl.GetPrimaryProvider()
	if err != nil {
		logger.L().Error("Failed to get primary provider for session replay adapter", zap.Error(err))
		return
	}
	storage.InitializeAdapter(provider)

	logger.L().Info("Storage service initialization completed",
		zap.Int("total_configs", len(configs)),
		zap.Int("successful_providers", successCount),
		zap.String("primary_provider", storageImpl.primary))
}

// loadOrCreateStorageConfigs loads existing configurations or creates default configuration
func loadOrCreateStorageConfigs(ctx context.Context, s *storageService) ([]*model.StorageConfig, error) {
	configs, err := s.GetStorageConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load storage configurations: %w", err)
	}

	if len(configs) == 0 {
		logger.L().Info("No storage configurations found, creating default local storage")
		defaultConfig := &model.StorageConfig{
			Name:        "default-local",
			Type:        model.StorageTypeLocal,
			Enabled:     true,
			Priority:    10,
			IsPrimary:   true,
			Description: "Default local storage for file operations with date hierarchy",
			Config: model.StorageConfigMap{
				"base_path":       config.Cfg.Session.ReplayDir,
				"path_strategy":   "date_hierarchy",
				"retention_days":  "30",
				"archive_days":    "7",
				"cleanup_enabled": "true",
				"archive_enabled": "true",
			},
		}

		if err := s.CreateStorageConfig(ctx, defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create default storage configuration: %w", err)
		}
		configs = []*model.StorageConfig{defaultConfig}
		logger.L().Info("Created default local storage configuration successfully")
	}

	return configs, nil
}

// validateStorageConfigs validates the status of storage configurations
func validateStorageConfigs(configs []*model.StorageConfig) {
	var enabledCount, primaryCount int
	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		enabledCount++
		logger.L().Info("Found enabled storage provider",
			zap.String("name", config.Name),
			zap.String("type", string(config.Type)),
			zap.Bool("is_primary", config.IsPrimary))

		if config.IsPrimary {
			primaryCount++
		}
	}

	if enabledCount == 0 {
		logger.L().Warn("No enabled storage providers found")
	}
	if primaryCount == 0 {
		logger.L().Warn("No primary storage provider configured")
	} else if primaryCount > 1 {
		logger.L().Warn("Multiple primary storage providers found", zap.Int("count", primaryCount))
	}
}

// initializeStorageProviders initializes all enabled storage providers
func initializeStorageProviders(ctx context.Context, s *storageService, configs []*model.StorageConfig) int {
	var successCount int
	for _, config := range configs {
		if !config.Enabled {
			logger.L().Debug("Skipping disabled storage provider", zap.String("name", config.Name))
			continue
		}

		provider, err := s.CreateProvider(config)
		if err != nil {
			logger.L().Warn("Failed to initialize storage provider",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)),
				zap.Error(err))
			continue
		}

		if err := provider.HealthCheck(ctx); err != nil {
			logger.L().Warn("Storage provider failed health check",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)),
				zap.Error(err))
			continue
		}

		s.providers[config.Name] = provider
		successCount++

		if config.IsPrimary {
			s.primary = config.Name
			logger.L().Info("Set primary storage provider",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)))
		}
	}

	return successCount
}

// verifyPrimaryProvider verifies the availability of the primary storage provider
func verifyPrimaryProvider(ctx context.Context, s *storageService) error {
	provider, err := s.GetPrimaryProvider()
	if err != nil {
		return fmt.Errorf("no primary storage provider available: %w", err)
	}

	if err := provider.HealthCheck(ctx); err != nil {
		logger.L().Warn("Primary storage provider health check failed",
			zap.String("type", provider.Type()),
			zap.Error(err))
		return err
	}

	logger.L().Info("Primary storage provider health check passed",
		zap.String("type", provider.Type()))
	return nil
}

func init() {
	DefaultStorageService = NewStorageService()

	// Start background storage health monitoring
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			if DefaultStorageService != nil {
				performHealthMonitoring()
			}
		}
	}()
}

// performHealthMonitoring performs periodic health checks on all storage providers
func performHealthMonitoring() {
	ctx := context.Background()
	storageImpl, ok := DefaultStorageService.(*storageService)
	if !ok {
		return
	}

	healthResults := make(map[string]error)

	for name, provider := range storageImpl.providers {
		if err := provider.HealthCheck(ctx); err != nil {
			healthResults[name] = err
			logger.L().Warn("Storage provider health check failed",
				zap.String("name", name),
				zap.String("type", provider.Type()),
				zap.Error(err))
		} else {
			healthResults[name] = nil
			logger.L().Debug("Storage provider health check passed",
				zap.String("name", name),
				zap.String("type", provider.Type()))
		}
	}

	// Log summary of health check results
	healthyCount := 0
	totalCount := len(healthResults)

	for _, err := range healthResults {
		if err == nil {
			healthyCount++
		}
	}

	if totalCount > 0 {
		logger.L().Info("Storage health monitoring completed",
			zap.Int("healthy_providers", healthyCount),
			zap.Int("total_providers", totalCount))
	}
}
