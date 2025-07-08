package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
)

// ICommandTemplateRepository defines the interface for command template data access
type ICommandTemplateRepository interface {
	Create(ctx context.Context, template *model.CommandTemplate) error
	GetByID(ctx context.Context, id int) (*model.CommandTemplate, error)
	GetByName(ctx context.Context, name string) (*model.CommandTemplate, error)
	List(ctx context.Context, offset, limit int, category string, builtin *bool) ([]*model.CommandTemplate, int64, error)
	Update(ctx context.Context, template *model.CommandTemplate) error
	Delete(ctx context.Context, id int) error
	GetBuiltInTemplates(ctx context.Context) ([]*model.CommandTemplate, error)
}

// CommandTemplateRepository implements ICommandTemplateRepository
type CommandTemplateRepository struct {
	db *gorm.DB
}

// NewCommandTemplateRepository creates a new command template repository
func NewCommandTemplateRepository(db *gorm.DB) ICommandTemplateRepository {
	return &CommandTemplateRepository{db: db}
}

// Create creates a new command template
func (r *CommandTemplateRepository) Create(ctx context.Context, template *model.CommandTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

// GetByID retrieves a command template by ID
func (r *CommandTemplateRepository) GetByID(ctx context.Context, id int) (*model.CommandTemplate, error) {
	var template model.CommandTemplate
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// GetByName retrieves a command template by name
func (r *CommandTemplateRepository) GetByName(ctx context.Context, name string) (*model.CommandTemplate, error) {
	var template model.CommandTemplate
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// List retrieves command templates with pagination and filters
func (r *CommandTemplateRepository) List(ctx context.Context, offset, limit int, category string, builtin *bool) ([]*model.CommandTemplate, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CommandTemplate{})

	// Apply filters
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if builtin != nil {
		query = query.Where("is_builtin = ?", *builtin)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	var templates []*model.CommandTemplate
	err := query.Order("is_builtin DESC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&templates).Error

	return templates, total, err
}

// Update updates an existing command template
func (r *CommandTemplateRepository) Update(ctx context.Context, template *model.CommandTemplate) error {
	// Don't allow updating built-in templates
	if template.IsBuiltin {
		return errors.New("cannot update built-in command template")
	}
	return r.db.WithContext(ctx).Save(template).Error
}

// Delete soft deletes a command template
func (r *CommandTemplateRepository) Delete(ctx context.Context, id int) error {
	// Check if it's a built-in template
	var template model.CommandTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error; err != nil {
		return err
	}

	if template.IsBuiltin {
		return errors.New("cannot delete built-in command template")
	}

	return r.db.WithContext(ctx).Delete(&model.CommandTemplate{}, id).Error
}

// GetBuiltInTemplates retrieves all built-in command templates
func (r *CommandTemplateRepository) GetBuiltInTemplates(ctx context.Context) ([]*model.CommandTemplate, error) {
	var templates []*model.CommandTemplate
	err := r.db.WithContext(ctx).Where("is_builtin = ?", true).
		Order("category, id").
		Find(&templates).Error
	return templates, err
}
