package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/repository"
)

// BaseService defines the interface for base service operations
type BaseService interface {
	// ExecuteInTransaction executes operations within a transaction
	ExecuteInTransaction(ctx context.Context, fn func(*gorm.DB) error) error

	// GetById retrieves a model by ID
	GetById(ctx context.Context, id int, model any) error

	// Create creates a model
	Create(ctx context.Context, model any) error

	// Update updates a model
	Update(ctx context.Context, model any, selects []string, omits []string) error

	// Delete deletes a model
	Delete(ctx context.Context, model any, id int) error
}

// baseService implements BaseService
type baseService struct {
	baseRepo       repository.BaseRepository
	historyService *HistoryService
}

// NewBaseService creates a new base service
func NewBaseService() BaseService {
	return &baseService{
		baseRepo:       repository.NewBaseRepository(),
		historyService: NewHistoryService(),
	}
}

// ExecuteInTransaction executes operations within a transaction
func (s *baseService) ExecuteInTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return s.baseRepo.ExecuteInTransaction(ctx, fn)
}

// GetById retrieves a model by ID
func (s *baseService) GetById(ctx context.Context, id int, model any) error {
	return s.baseRepo.GetById(ctx, id, model)
}

// Create creates a model
func (s *baseService) Create(ctx context.Context, model any) error {
	return s.baseRepo.Create(ctx, model)
}

// Update updates a model
func (s *baseService) Update(ctx context.Context, model any, selects []string, omits []string) error {
	return s.baseRepo.Update(ctx, model, selects, omits)
}

// Delete deletes a model
func (s *baseService) Delete(ctx context.Context, model any, id int) error {
	return s.baseRepo.Delete(ctx, model, id)
}
