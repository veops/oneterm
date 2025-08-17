package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
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

	// Clear all primary flags for ensuring single primary constraint
	ClearAllPrimaryFlags(ctx context.Context) error

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
	ListRDPFiles(ctx context.Context, assetId int, directory string, limit, offset int) ([]*model.FileMetadata, int64, error)

	// Provider management
	GetPrimaryProvider() (storage.Provider, error)
	HealthCheck(ctx context.Context) map[string]error
	CreateProvider(config *model.StorageConfig) (storage.Provider, error)
	RefreshProviders(ctx context.Context) error

	// New method for building queries
	BuildQuery(ctx *gin.Context) *gorm.DB

	// GetAvailableProvider returns an available storage provider with fallback logic
	// Priority: Primary storage first, then by priority (lower number = higher priority)
	GetAvailableProvider(ctx context.Context) (storage.Provider, error)

	// Storage Metrics Operations
	GetStorageMetrics(ctx context.Context) (map[string]*model.StorageMetrics, error)
	RefreshStorageMetrics(ctx context.Context) error
	CalculateStorageMetrics(ctx context.Context, storageName string) (*model.StorageMetrics, error)
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

	// Only initialize storage provider if it's enabled
	if config.Enabled {
		provider, err := s.CreateProvider(config)
		if err != nil {
			logger.L().Warn("Failed to initialize storage provider",
				zap.String("name", config.Name),
				zap.Error(err))
			return nil // Don't fail the creation, just warn
		}

		// Perform health check before adding to providers map
		if err := provider.HealthCheck(ctx); err != nil {
			logger.L().Warn("Storage provider failed health check",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)),
				zap.Error(err))
			// Still add to providers map even if health check fails,
			// so it appears in health status for monitoring
		}

		s.providers[config.Name] = provider
		if config.IsPrimary {
			s.primary = config.Name
			logger.L().Info("Set new storage as primary provider",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)))
		}

		logger.L().Info("Storage provider initialized successfully",
			zap.String("name", config.Name),
			zap.String("type", string(config.Type)),
			zap.Bool("is_primary", config.IsPrimary))
	} else {
		logger.L().Info("Storage configuration created but not initialized (disabled)",
			zap.String("name", config.Name),
			zap.String("type", string(config.Type)))
	}

	return nil
}

func (s *storageService) UpdateStorageConfig(ctx context.Context, config *model.StorageConfig) error {
	if err := s.validateConfig(config); err != nil {
		return err
	}

	if err := s.storageRepo.UpdateStorageConfig(ctx, config); err != nil {
		return err
	}

	if err := s.RefreshProviders(ctx); err != nil {
		logger.L().Warn("Failed to refresh providers after config update", zap.Error(err))
	}

	return nil
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

	// Get all storage configurations from database
	configs, err := s.GetStorageConfigs(ctx)
	if err != nil {
		logger.L().Error("Failed to get storage configs for health check", zap.Error(err))
		return results
	}

	// Check each configuration
	for _, config := range configs {
		if !config.Enabled {
			// For disabled configs, add a special error indicating they are disabled
			results[config.Name] = fmt.Errorf("storage provider is disabled")
			continue
		}

		// For enabled configs, check if provider exists and perform health check
		if provider, exists := s.providers[config.Name]; exists {
			err := provider.HealthCheck(ctx)
			if err != nil {
				// Add more context to the error message
				logger.L().Warn("Storage provider health check failed",
					zap.String("name", config.Name),
					zap.String("type", string(config.Type)),
					zap.Error(err))
				results[config.Name] = fmt.Errorf("health check failed: %v", err)
			} else {
				results[config.Name] = nil
			}
		} else {
			// Provider should exist but doesn't - this indicates an initialization problem
			logger.L().Warn("Storage provider not found in memory",
				zap.String("name", config.Name),
				zap.String("type", string(config.Type)),
				zap.Bool("enabled", config.Enabled))
			results[config.Name] = fmt.Errorf("storage provider not initialized, possible configuration error or initialization failure")
		}
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
		s3Config, err := providers.ParseS3ConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse S3 config: %w", err)
		}
		return providers.NewS3(s3Config)

	case model.StorageTypeAzure:
		azureConfig, err := providers.ParseAzureConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Azure config: %w", err)
		}
		return providers.NewAzure(azureConfig)

	case model.StorageTypeCOS:
		cosConfig, err := providers.ParseCOSConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse COS config: %w", err)
		}
		return providers.NewCOS(cosConfig)

	case model.StorageTypeOSS:
		ossConfig, err := providers.ParseOSSConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OSS config: %w", err)
		}
		return providers.NewOSS(ossConfig)

	case model.StorageTypeOBS:
		obsConfig, err := providers.ParseOBSConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OBS config: %w", err)
		}
		return providers.NewOBS(obsConfig)

	case model.StorageTypeOOS:
		oosConfig, err := providers.ParseOOSConfigFromMap(config.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OOS config: %w", err)
		}
		return providers.NewOOS(oosConfig)

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

// Global context for health monitoring
var (
	healthMonitoringCtx    context.Context
	healthMonitoringCancel context.CancelFunc
	healthMonitoringWg     sync.WaitGroup
)

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

	// Initialize health monitoring context
	healthMonitoringCtx, healthMonitoringCancel = context.WithCancel(context.Background())

	// Start background storage health monitoring
	healthMonitoringWg.Add(1)
	go func() {
		defer healthMonitoringWg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-healthMonitoringCtx.Done():
				logger.L().Info("Storage health monitoring stopped")
				return
			case <-ticker.C:
				if DefaultStorageService != nil {
					performHealthMonitoring()
				}
			}
		}
	}()

	// Start background storage metrics calculation
	// go func() {
	// 	// Update storage metrics every 30 minutes to avoid high resource consumption
	// 	ticker := time.NewTicker(30 * time.Minute)
	// 	defer ticker.Stop()

	// 	// Initial update after 5 minutes
	// 	time.Sleep(5 * time.Minute)
	// 	if DefaultStorageService != nil {
	// 		performMetricsUpdate()
	// 	}

	// 	for {
	// 		<-ticker.C
	// 		if DefaultStorageService != nil {
	// 			performMetricsUpdate()
	// 		}
	// 	}
	// }()
}

// StopStorageService stops all background tasks for storage service
func StopStorageService() {
	if healthMonitoringCancel != nil {
		healthMonitoringCancel()
		healthMonitoringWg.Wait()
		logger.L().Info("Storage service background tasks stopped")
	}
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

// performMetricsUpdate performs periodic storage metrics calculation
func performMetricsUpdate() {
	ctx := context.Background()
	storageImpl, ok := DefaultStorageService.(*storageService)
	if !ok {
		return
	}

	// Refresh metrics for all storage configurations
	if err := storageImpl.RefreshStorageMetrics(ctx); err != nil {
		logger.L().Warn("Failed to refresh storage metrics during background update", zap.Error(err))
		return
	}

	// Log completion
	configs, err := storageImpl.GetStorageConfigs(ctx)
	if err == nil && len(configs) > 0 {
		enabledCount := 0
		for _, config := range configs {
			if config.Enabled {
				enabledCount++
			}
		}
		logger.L().Info("Storage metrics update completed",
			zap.Int("enabled_storages", enabledCount),
			zap.Int("total_storages", len(configs)))
	}
}

func (s *storageService) RefreshProviders(ctx context.Context) error {
	s.providers = make(map[string]storage.Provider)
	s.primary = ""

	configs, err := s.GetStorageConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to load storage configurations: %w", err)
	}

	successCount := initializeStorageProviders(ctx, s, configs)

	if provider, err := s.GetPrimaryProvider(); err == nil {
		storage.InitializeAdapter(provider)
		logger.L().Info("Session replay adapter re-initialized with new primary provider",
			zap.String("provider_type", provider.Type()))
	} else {
		logger.L().Warn("Failed to re-initialize session replay adapter", zap.Error(err))
	}

	logger.L().Info("Storage providers refreshed",
		zap.Int("total_configs", len(configs)),
		zap.Int("successful_providers", successCount))

	return nil
}

func (s *storageService) ClearAllPrimaryFlags(ctx context.Context) error {
	if err := s.storageRepo.ClearAllPrimaryFlags(ctx); err != nil {
		return fmt.Errorf("failed to clear primary flags in database: %w", err)
	}

	s.primary = ""

	if err := s.RefreshProviders(ctx); err != nil {
		logger.L().Warn("Failed to refresh providers after clearing primary flags", zap.Error(err))
		return err
	}

	return nil
}

func (s *storageService) ListRDPFiles(ctx context.Context, assetId int, directory string, limit, offset int) ([]*model.FileMetadata, int64, error) {
	prefix := fmt.Sprintf("rdp_files/%d/%s", assetId, directory)
	return s.storageRepo.ListFileMetadata(ctx, prefix, limit, offset)
}

func (s *storageService) GetStorageMetrics(ctx context.Context) (map[string]*model.StorageMetrics, error) {
	metricsList, err := s.storageRepo.GetStorageMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage metrics: %w", err)
	}

	metricsMap := make(map[string]*model.StorageMetrics)
	for _, metric := range metricsList {
		metricsMap[metric.StorageName] = metric
	}

	return metricsMap, nil
}

func (s *storageService) RefreshStorageMetrics(ctx context.Context) error {
	// Get all storage configurations
	configs, err := s.GetStorageConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage configs: %w", err)
	}

	// Calculate metrics for each storage
	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		metric, err := s.CalculateStorageMetrics(ctx, config.Name)
		if err != nil {
			logger.L().Warn("Failed to calculate storage metrics",
				zap.String("storage", config.Name),
				zap.Error(err))
			continue
		}

		// Upsert metrics
		if err := s.storageRepo.UpsertStorageMetrics(ctx, metric); err != nil {
			logger.L().Warn("Failed to save storage metrics",
				zap.String("storage", config.Name),
				zap.Error(err))
		}
	}

	return nil
}

func (s *storageService) CalculateStorageMetrics(ctx context.Context, storageName string) (*model.StorageMetrics, error) {
	metric := &model.StorageMetrics{
		StorageName: storageName,
		LastUpdated: time.Now(),
		IsHealthy:   true,
	}

	// Check if provider exists and is healthy
	if provider, exists := s.providers[storageName]; exists {
		if err := provider.HealthCheck(ctx); err != nil {
			metric.IsHealthy = false
			metric.ErrorMessage = err.Error()
		}
	} else {
		metric.IsHealthy = false
		metric.ErrorMessage = "Provider not initialized"
	}

	// Calculate file counts and sizes efficiently using database aggregation
	// Use storage_name field from file_metadata table
	if err := s.calculateFileStats(ctx, storageName, metric); err != nil {
		logger.L().Warn("Failed to calculate file stats",
			zap.String("storage", storageName),
			zap.Error(err))
		// Don't fail completely, just log the warning
	}

	return metric, nil
}

// Helper method to calculate file statistics efficiently
func (s *storageService) calculateFileStats(ctx context.Context, storageName string, metric *model.StorageMetrics) error {

	// Calculate total file count and size
	type Result struct {
		Count int64 `json:"count"`
		Size  int64 `json:"size"`
	}

	var totalResult Result
	err := dbpkg.DB.Model(&model.FileMetadata{}).
		Select("COUNT(*) as count, COALESCE(SUM(file_size), 0) as size").
		Where("storage_name = ?", storageName).
		Scan(&totalResult).Error
	if err != nil {
		return fmt.Errorf("failed to calculate total stats: %w", err)
	}

	metric.FileCount = totalResult.Count
	metric.TotalSize = totalResult.Size

	// Calculate replay-specific stats
	var replayResult Result
	err = dbpkg.DB.Model(&model.FileMetadata{}).
		Select("COUNT(*) as count, COALESCE(SUM(file_size), 0) as size").
		Where("storage_name = ? AND category = ?", storageName, "replay").
		Scan(&replayResult).Error
	if err != nil {
		logger.L().Warn("Failed to calculate replay stats", zap.Error(err))
	} else {
		metric.ReplayCount = replayResult.Count
		metric.ReplaySize = replayResult.Size
	}

	// Calculate RDP file stats
	var rdpResult Result
	err = dbpkg.DB.Model(&model.FileMetadata{}).
		Select("COUNT(*) as count, COALESCE(SUM(file_size), 0) as size").
		Where("storage_name = ? AND category = ?", storageName, "rdp_file").
		Scan(&rdpResult).Error
	if err != nil {
		logger.L().Warn("Failed to calculate RDP file stats", zap.Error(err))
	} else {
		metric.RdpFileCount = rdpResult.Count
		metric.RdpFileSize = rdpResult.Size
	}

	return nil
}
