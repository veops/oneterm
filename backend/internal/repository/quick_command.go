package repository

import (
	"context"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

type QuickCommand struct{}

var DefaultQuickCommand = NewQuickCommand()

func NewQuickCommand() *QuickCommand {
	return &QuickCommand{}
}

// BuildQuery builds the base query for quick commands
func (r *QuickCommand) BuildQuery(ctx context.Context) *gorm.DB {
	return dbpkg.DB.Model(&model.QuickCommand{})
}

// Create creates a new quick command
func (r *QuickCommand) Create(ctx context.Context, cmd *model.QuickCommand) error {
	return dbpkg.DB.Create(cmd).Error
}

// GetUserCommands retrieves all quick commands visible to the user
func (r *QuickCommand) GetUserCommands(ctx context.Context, userId int) ([]*model.QuickCommand, error) {
	var cmds []*model.QuickCommand
	err := dbpkg.DB.Where("creator_id = ? OR is_global = ?", userId, true).Find(&cmds).Error
	return cmds, err
}

// GetById retrieves a quick command by its ID
func (r *QuickCommand) GetById(ctx context.Context, id int) (*model.QuickCommand, error) {
	var cmd model.QuickCommand
	err := dbpkg.DB.First(&cmd, id).Error
	return &cmd, err
}

// Delete deletes a quick command by its ID
func (r *QuickCommand) Delete(ctx context.Context, id int) error {
	return dbpkg.DB.Delete(&model.QuickCommand{}, id).Error
}

// Update updates an existing quick command
func (r *QuickCommand) Update(ctx context.Context, cmd *model.QuickCommand) error {
	return dbpkg.DB.Save(cmd).Error
}
