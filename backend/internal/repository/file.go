package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
)

// IFileRepository file history repository interface
type IFileRepository interface {
	AddFileHistory(ctx context.Context, history *model.FileHistory) error
	GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error)
}

// FileRepository file history repository implementation
type FileRepository struct {
	db *gorm.DB
}

// NewFileRepository creates a file history repository
func NewFileRepository(db *gorm.DB) IFileRepository {
	return &FileRepository{
		db: db,
	}
}

// AddFileHistory adds a file history record
func (r *FileRepository) AddFileHistory(ctx context.Context, history *model.FileHistory) error {
	return r.db.Create(history).Error
}

// GetFileHistory gets file history records
func (r *FileRepository) GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error) {
	db := r.db.Model(&model.FileHistory{})

	// Apply filter conditions
	for key, value := range filters {
		if value != nil && value != "" {
			db = db.Where(key, value)
		}
	}

	// Count total records
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Query records
	var histories []*model.FileHistory
	if err := db.Find(&histories).Error; err != nil {
		return nil, 0, err
	}

	return histories, count, nil
}
