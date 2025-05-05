package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
)

// CommandRepository defines the interface for command repository
type CommandRepository interface {
	// Add any repository-specific methods here
	GetCommand(ctx context.Context, id int) (*model.Command, error)
	ListCommands(ctx context.Context, filters map[string]interface{}) ([]*model.Command, error)
}

type commandRepository struct{}

// NewCommandRepository creates a new command repository
func NewCommandRepository() CommandRepository {
	return &commandRepository{}
}

// GetCommand retrieves a command by ID
func (r *commandRepository) GetCommand(ctx context.Context, id int) (*model.Command, error) {
	command := &model.Command{}
	if err := dbpkg.DB.Where("id = ?", id).First(command).Error; err != nil {
		return nil, err
	}
	return command, nil
}

// ListCommands lists commands based on filters
func (r *commandRepository) ListCommands(ctx context.Context, filters map[string]interface{}) ([]*model.Command, error) {
	var commands []*model.Command
	db := dbpkg.DB.Model(&model.Command{})

	for key, value := range filters {
		db = db.Where(key, value)
	}

	if err := db.Find(&commands).Error; err != nil {
		return nil, err
	}

	return commands, nil
}
