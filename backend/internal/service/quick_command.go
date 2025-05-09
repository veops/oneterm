package service

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

type QuickCommand struct {
	repo *repository.QuickCommand
}

var DefaultQuickCommand = NewQuickCommand()

func NewQuickCommand() *QuickCommand {
	return &QuickCommand{
		repo: repository.DefaultQuickCommand,
	}
}

// BuildQuery builds the base query for quick commands
func (s *QuickCommand) BuildQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.QuickCommand{})

	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "command")

	return db
}

// Create creates a new quick command
func (s *QuickCommand) Create(ctx context.Context, cmd *model.QuickCommand) error {
	return s.repo.Create(ctx, cmd)
}

// GetUserCommands retrieves all quick commands visible to the user
func (s *QuickCommand) GetUserCommands(ctx context.Context, userId int) ([]*model.QuickCommand, error) {
	return s.repo.GetUserCommands(ctx, userId)
}

// GetById retrieves a quick command by its ID
func (s *QuickCommand) GetById(ctx context.Context, id int) (*model.QuickCommand, error) {
	return s.repo.GetById(ctx, id)
}

// Delete deletes a quick command by its ID
func (s *QuickCommand) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

// Update updates an existing quick command
func (s *QuickCommand) Update(ctx context.Context, cmd *model.QuickCommand) error {
	return s.repo.Update(ctx, cmd)
}
