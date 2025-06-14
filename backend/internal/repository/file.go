package repository

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
)

// IFileRepository file history repository interface
type IFileRepository interface {
	AddFileHistory(ctx context.Context, history *model.FileHistory) error
	BuildFileHistoryQuery(ctx *gin.Context) *gorm.DB
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

// BuildFileHistoryQuery builds a query for file history records
func (r *FileRepository) BuildFileHistoryQuery(ctx *gin.Context) *gorm.DB {
	return r.db.Model(&model.FileHistory{})
}
