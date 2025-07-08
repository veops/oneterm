package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// CommandTemplateService handles business logic for command templates
type CommandTemplateService struct {
	repo repository.ICommandTemplateRepository
}

// NewCommandTemplateService creates a new command template service
func NewCommandTemplateService() *CommandTemplateService {
	repo := repository.NewCommandTemplateRepository(dbpkg.DB)
	return &CommandTemplateService{
		repo: repo,
	}
}

// BuildQuery builds the base query for command templates
func (s *CommandTemplateService) BuildQuery(ctx context.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultCommandTemplate)
	return db, nil
}

// CreateCommandTemplate creates a new command template
func (s *CommandTemplateService) CreateCommandTemplate(ctx context.Context, template *model.CommandTemplate) error {
	// Validate the template
	if err := s.ValidateCommandTemplate(template); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check for duplicate names
	existing, err := s.repo.GetByName(ctx, template.Name)
	if err != nil {
		return fmt.Errorf("failed to check existing template: %w", err)
	}
	if existing != nil {
		return errors.New("template with this name already exists")
	}

	// Set default values
	template.IsBuiltin = false

	return s.repo.Create(ctx, template)
}

// GetCommandTemplate retrieves a command template by ID
func (s *CommandTemplateService) GetCommandTemplate(ctx context.Context, id int) (*model.CommandTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

// GetCommandTemplateByName retrieves a command template by name
func (s *CommandTemplateService) GetCommandTemplateByName(ctx context.Context, name string) (*model.CommandTemplate, error) {
	return s.repo.GetByName(ctx, name)
}

// ListCommandTemplates retrieves command templates with pagination and filters
func (s *CommandTemplateService) ListCommandTemplates(ctx context.Context, offset, limit int, category string, builtin *bool) ([]*model.CommandTemplate, int64, error) {
	return s.repo.List(ctx, offset, limit, category, builtin)
}

// UpdateCommandTemplate updates an existing command template
func (s *CommandTemplateService) UpdateCommandTemplate(ctx context.Context, template *model.CommandTemplate) error {
	// Validate the template
	if err := s.ValidateCommandTemplate(template); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if template exists
	existing, err := s.repo.GetByID(ctx, template.Id)
	if err != nil {
		return fmt.Errorf("failed to get existing template: %w", err)
	}
	if existing == nil {
		return errors.New("command template not found")
	}

	// Don't allow changing built-in status
	template.IsBuiltin = existing.IsBuiltin

	return s.repo.Update(ctx, template)
}

// DeleteCommandTemplate deletes a command template
func (s *CommandTemplateService) DeleteCommandTemplate(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

// GetBuiltInTemplates retrieves all built-in command templates
func (s *CommandTemplateService) GetBuiltInTemplates(ctx context.Context) ([]*model.CommandTemplate, error) {
	return s.repo.GetBuiltInTemplates(ctx)
}

// ValidateCommandTemplate validates a command template
func (s *CommandTemplateService) ValidateCommandTemplate(template *model.CommandTemplate) error {
	if template.Name == "" {
		return errors.New("template name is required")
	}

	if len(template.Name) > 128 {
		return errors.New("template name too long (max 128 characters)")
	}

	if template.Category == "" {
		return errors.New("template category is required")
	}

	// Validate category
	validCategories := []string{"security", "system", "database", "network", "file", "developer", "custom"}
	categoryValid := false
	for _, cat := range validCategories {
		if string(template.Category) == cat {
			categoryValid = true
			break
		}
	}
	if !categoryValid {
		return fmt.Errorf("invalid category: %s. Valid categories: %v", template.Category, validCategories)
	}

	// Validate command IDs if provided
	if len(template.CmdIds) > 0 {
		if err := s.validateCommandIds(template.CmdIds); err != nil {
			return fmt.Errorf("invalid command IDs: %w", err)
		}
	}

	return nil
}

// validateCommandIds validates that all command IDs exist
func (s *CommandTemplateService) validateCommandIds(cmdIds model.Slice[int]) error {
	if len(cmdIds) == 0 {
		return nil
	}

	// Convert model.Slice[int] to []int for SQL query
	ids := make([]int, len(cmdIds))
	copy(ids, cmdIds)

	// Check if all command IDs exist
	var count int64
	err := dbpkg.DB.Model(&model.Command{}).Where("id IN ?", ids).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to validate command IDs: %w", err)
	}

	if int(count) != len(cmdIds) {
		return errors.New("some command IDs do not exist")
	}

	return nil
}

// GetTemplateCommands retrieves all commands for a template
func (s *CommandTemplateService) GetTemplateCommands(ctx context.Context, templateId int) ([]*model.Command, error) {
	template, err := s.repo.GetByID(ctx, templateId)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("command template not found")
	}

	if len(template.CmdIds) == 0 {
		return []*model.Command{}, nil
	}

	// Convert model.Slice[int] to []int for SQL query
	ids := make([]int, len(template.CmdIds))
	copy(ids, template.CmdIds)

	var commands []*model.Command
	err = dbpkg.DB.WithContext(ctx).Where("id IN ?", ids).Find(&commands).Error
	return commands, err
}
