package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
)

// ITimeTemplateRepository defines the interface for time template data access
type ITimeTemplateRepository interface {
	Create(ctx context.Context, template *model.TimeTemplate) error
	GetByID(ctx context.Context, id int) (*model.TimeTemplate, error)
	GetByName(ctx context.Context, name string) (*model.TimeTemplate, error)
	List(ctx context.Context, offset, limit int, category string, active *bool) ([]*model.TimeTemplate, int64, error)
	Update(ctx context.Context, template *model.TimeTemplate) error
	Delete(ctx context.Context, id int) error
	IncrementUsage(ctx context.Context, id int) error
	GetBuiltInTemplates(ctx context.Context) ([]*model.TimeTemplate, error)
	InitBuiltInTemplates(ctx context.Context) error
}

// TimeTemplateRepository implements ITimeTemplateRepository
type TimeTemplateRepository struct {
	db *gorm.DB
}

// NewTimeTemplateRepository creates a new time template repository
func NewTimeTemplateRepository(db *gorm.DB) ITimeTemplateRepository {
	return &TimeTemplateRepository{db: db}
}

// Create creates a new time template
func (r *TimeTemplateRepository) Create(ctx context.Context, template *model.TimeTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

// GetByID retrieves a time template by ID
func (r *TimeTemplateRepository) GetByID(ctx context.Context, id int) (*model.TimeTemplate, error) {
	var template model.TimeTemplate
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// GetByName retrieves a time template by name
func (r *TimeTemplateRepository) GetByName(ctx context.Context, name string) (*model.TimeTemplate, error) {
	var template model.TimeTemplate
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// List retrieves time templates with pagination and filters
func (r *TimeTemplateRepository) List(ctx context.Context, offset, limit int, category string, active *bool) ([]*model.TimeTemplate, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.TimeTemplate{})

	// Apply filters
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if active != nil {
		query = query.Where("is_active = ?", *active)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	var templates []*model.TimeTemplate
	err := query.Order("is_builtin DESC, usage_count DESC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&templates).Error

	return templates, total, err
}

// Update updates an existing time template
func (r *TimeTemplateRepository) Update(ctx context.Context, template *model.TimeTemplate) error {
	// Don't allow updating built-in templates
	if template.IsBuiltIn {
		return errors.New("cannot update built-in time template")
	}
	return r.db.WithContext(ctx).Save(template).Error
}

// Delete soft deletes a time template
func (r *TimeTemplateRepository) Delete(ctx context.Context, id int) error {
	// Check if it's a built-in template
	var template model.TimeTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error; err != nil {
		return err
	}

	if template.IsBuiltIn {
		return errors.New("cannot delete built-in time template")
	}

	return r.db.WithContext(ctx).Delete(&model.TimeTemplate{}, id).Error
}

// IncrementUsage increments the usage count of a time template
func (r *TimeTemplateRepository) IncrementUsage(ctx context.Context, id int) error {
	return r.db.WithContext(ctx).Model(&model.TimeTemplate{}).
		Where("id = ?", id).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}

// GetBuiltInTemplates retrieves all built-in time templates
func (r *TimeTemplateRepository) GetBuiltInTemplates(ctx context.Context) ([]*model.TimeTemplate, error) {
	var templates []*model.TimeTemplate
	err := r.db.WithContext(ctx).Where("is_builtin = ?", true).
		Order("category, id").
		Find(&templates).Error
	return templates, err
}

// InitBuiltInTemplates initializes built-in time templates
func (r *TimeTemplateRepository) InitBuiltInTemplates(ctx context.Context) error {
	// Check if built-in templates already exist
	var count int64
	r.db.WithContext(ctx).Model(&model.TimeTemplate{}).Where("is_builtin = ?", true).Count(&count)

	if count > 0 {
		// Built-in templates already exist, skip initialization
		return nil
	}

	// Create built-in templates
	for _, template := range model.BuiltInTimeTemplates {
		// Create a copy to avoid modifying the original
		newTemplate := template
		if err := r.db.WithContext(ctx).Create(&newTemplate).Error; err != nil {
			return err
		}
	}

	return nil
}
