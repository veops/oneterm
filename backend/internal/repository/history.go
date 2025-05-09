package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// HistoryRepository defines the interface for history repository
type HistoryRepository interface {
	GetHistories(ctx context.Context, filters map[string]any) ([]*model.History, error)
	GetHistory(ctx context.Context, id int) (*model.History, error)
	CreateHistory(ctx context.Context, history *model.History) error
}

type historyRepository struct{}

// NewHistoryRepository creates a new history repository
func NewHistoryRepository() HistoryRepository {
	return &historyRepository{}
}

// GetHistory retrieves a history record by ID
func (r *historyRepository) GetHistory(ctx context.Context, id int) (*model.History, error) {
	history := &model.History{}
	if err := dbpkg.DB.Where("id = ?", id).First(history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// GetHistories retrieves history records based on filters
func (r *historyRepository) GetHistories(ctx context.Context, filters map[string]any) ([]*model.History, error) {
	var histories []*model.History
	db := dbpkg.DB.Model(&model.History{})

	for key, value := range filters {
		db = db.Where(key, value)
	}

	if err := db.Find(&histories).Error; err != nil {
		return nil, err
	}

	return histories, nil
}

// CreateHistory creates a new history record
func (r *historyRepository) CreateHistory(ctx context.Context, history *model.History) error {
	return dbpkg.DB.Create(history).Error
}
