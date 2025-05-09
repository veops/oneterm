package repository

import (
	"context"

	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// BaseRepository defines the interface for basic repository operations
type BaseRepository interface {
	// GetById retrieves a model by ID
	GetById(ctx context.Context, id int, model any) error

	// Create creates a new model
	Create(ctx context.Context, model any) error

	// Update updates a model
	Update(ctx context.Context, model any, selects []string, omits []string) error

	// Delete deletes a model by ID
	Delete(ctx context.Context, model any, id int) error

	// ExecuteInTransaction executes operations within a transaction
	ExecuteInTransaction(ctx context.Context, fn func(*gorm.DB) error) error
}

// baseRepository implements BaseRepository
type baseRepository struct{}

// NewBaseRepository creates a new base repository
func NewBaseRepository() BaseRepository {
	return &baseRepository{}
}

// GetById retrieves a model by ID
func (r *baseRepository) GetById(ctx context.Context, id int, model any) error {
	return dbpkg.DB.Model(model).Where("id = ?", id).First(model).Error
}

// Create creates a new model
func (r *baseRepository) Create(ctx context.Context, model any) error {
	return dbpkg.DB.Create(model).Error
}

// Update updates a model
func (r *baseRepository) Update(ctx context.Context, model any, selects []string, omits []string) error {
	db := dbpkg.DB
	if len(selects) > 0 {
		db = db.Select(selects)
	}
	if len(omits) > 0 {
		db = db.Omit(omits...)
	}
	return db.Save(model).Error
}

// Delete deletes a model by ID
func (r *baseRepository) Delete(ctx context.Context, model any, id int) error {
	return dbpkg.DB.Delete(model, id).Error
}

// ExecuteInTransaction executes operations within a transaction
func (r *baseRepository) ExecuteInTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return dbpkg.DB.Transaction(fn)
}
