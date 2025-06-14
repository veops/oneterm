package repository

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// StorageRepository defines interface for storage-related database operations
type StorageRepository interface {
	BaseRepository

	// BuildQuery builds query for storage configurations with filters
	BuildQuery(ctx *gin.Context) *gorm.DB

	// GetStorageConfigs retrieves all storage configurations
	GetStorageConfigs(ctx context.Context) ([]*model.StorageConfig, error)

	// GetStorageConfigByName retrieves storage config by name
	GetStorageConfigByName(ctx context.Context, name string) (*model.StorageConfig, error)

	// CreateStorageConfig creates a new storage configuration
	CreateStorageConfig(ctx context.Context, config *model.StorageConfig) error

	// UpdateStorageConfig updates storage configuration
	UpdateStorageConfig(ctx context.Context, config *model.StorageConfig) error

	// DeleteStorageConfig deletes storage configuration
	DeleteStorageConfig(ctx context.Context, name string) error

	// GetFileMetadata retrieves file metadata
	GetFileMetadata(ctx context.Context, key string) (*model.FileMetadata, error)

	// CreateFileMetadata creates file metadata record
	CreateFileMetadata(ctx context.Context, metadata *model.FileMetadata) error

	// UpdateFileMetadata updates file metadata
	UpdateFileMetadata(ctx context.Context, metadata *model.FileMetadata) error

	// DeleteFileMetadata deletes file metadata
	DeleteFileMetadata(ctx context.Context, key string) error

	// ListFileMetadata lists file metadata with pagination
	ListFileMetadata(ctx context.Context, prefix string, limit, offset int) ([]*model.FileMetadata, int64, error)
}

// storageRepository implements StorageRepository
type storageRepository struct {
	*baseRepository
}

// NewStorageRepository creates a new storage repository
func NewStorageRepository() StorageRepository {
	return &storageRepository{
		baseRepository: &baseRepository{},
	}
}

// BuildQuery builds query for storage configurations with filters
func (r *storageRepository) BuildQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.StorageConfig{})

	// Add search functionality
	if search := ctx.Query("search"); search != "" {
		db = db.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Add type filter
	if storageType := ctx.Query("type"); storageType != "" {
		db = db.Where("type = ?", storageType)
	}

	// Add enabled filter
	if enabled := ctx.Query("enabled"); enabled != "" {
		db = db.Where("enabled = ?", enabled == "true")
	}

	// Add primary filter
	if primary := ctx.Query("primary"); primary != "" {
		db = db.Where("is_primary = ?", primary == "true")
	}

	return db
}

// GetStorageConfigs retrieves all storage configurations
func (r *storageRepository) GetStorageConfigs(ctx context.Context) ([]*model.StorageConfig, error) {
	var configs []*model.StorageConfig
	err := dbpkg.DB.Find(&configs).Error
	return configs, err
}

// GetStorageConfigByName retrieves storage config by name
func (r *storageRepository) GetStorageConfigByName(ctx context.Context, name string) (*model.StorageConfig, error) {
	var config model.StorageConfig
	err := dbpkg.DB.Where("name = ?", name).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateStorageConfig creates a new storage configuration
func (r *storageRepository) CreateStorageConfig(ctx context.Context, config *model.StorageConfig) error {
	return dbpkg.DB.Create(config).Error
}

// UpdateStorageConfig updates storage configuration
func (r *storageRepository) UpdateStorageConfig(ctx context.Context, config *model.StorageConfig) error {
	return dbpkg.DB.Save(config).Error
}

// DeleteStorageConfig deletes storage configuration
func (r *storageRepository) DeleteStorageConfig(ctx context.Context, name string) error {
	return dbpkg.DB.Where("name = ?", name).Delete(&model.StorageConfig{}).Error
}

// GetFileMetadata retrieves file metadata
func (r *storageRepository) GetFileMetadata(ctx context.Context, key string) (*model.FileMetadata, error) {
	var metadata model.FileMetadata
	err := dbpkg.DB.Where("storage_key = ?", key).First(&metadata).Error
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

// CreateFileMetadata creates file metadata record
func (r *storageRepository) CreateFileMetadata(ctx context.Context, metadata *model.FileMetadata) error {
	return dbpkg.DB.Create(metadata).Error
}

// UpdateFileMetadata updates file metadata
func (r *storageRepository) UpdateFileMetadata(ctx context.Context, metadata *model.FileMetadata) error {
	return dbpkg.DB.Save(metadata).Error
}

// DeleteFileMetadata deletes file metadata
func (r *storageRepository) DeleteFileMetadata(ctx context.Context, key string) error {
	return dbpkg.DB.Where("storage_key = ?", key).Delete(&model.FileMetadata{}).Error
}

// ListFileMetadata lists file metadata with pagination
func (r *storageRepository) ListFileMetadata(ctx context.Context, prefix string, limit, offset int) ([]*model.FileMetadata, int64, error) {
	var metadata []*model.FileMetadata
	var total int64

	query := dbpkg.DB.Model(&model.FileMetadata{})
	if prefix != "" {
		query = query.Where("storage_key LIKE ?", prefix+"%")
	}

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get records with pagination
	err = query.Limit(limit).Offset(offset).Find(&metadata).Error
	return metadata, total, err
}
