package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// CommandService handles command business logic
type CommandService struct {
	repo repository.CommandRepository
}

// NewCommandService creates a new command service
func NewCommandService() *CommandService {
	return &CommandService{
		repo: repository.NewCommandRepository(),
	}
}

// CheckDependencies checks if command has dependent assets
func (s *CommandService) CheckDependencies(ctx context.Context, commandId int) (string, error) {
	assetName := ""
	err := dbpkg.DB.
		Model(model.DefaultAsset).
		Select("name").
		Where(fmt.Sprintf("JSON_CONTAINS(cmd_ids, '%d')", commandId)).
		First(&assetName).
		Error

	return assetName, err
}

// BuildQuery constructs command query with basic filters
func (s *CommandService) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(&model.Command{})

	// Apply filters
	db = dbpkg.FilterEqual(ctx, db, "id", "enable")
	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "cmd")

	// Handle IDs filter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Apply category filter
	if category := ctx.Query("category"); category != "" {
		db = db.Where("category = ?", category)
	}

	// Apply risk level filter
	if riskLevelStr := ctx.Query("risk_level"); riskLevelStr != "" {
		riskLevel := cast.ToInt(riskLevelStr)
		db = db.Where("risk_level = ?", riskLevel)
	}

	return db, nil
}

// GetAuthorizedCommandIds gets command IDs that the user is authorized to access using V2 authorization system
func (s *CommandService) GetAuthorizedCommandIds(ctx *gin.Context) ([]int, error) {
	// Use V2 authorization system to get authorized asset IDs
	authV2Service := NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	if err != nil {
		return nil, err
	}

	// No authorized assets means no authorized commands
	if len(assetIds) == 0 {
		return []int{}, nil
	}

	// Get command IDs from authorized assets
	cmdIds := make([]model.Slice[int], 0)
	if err = dbpkg.DB.
		Model(model.DefaultAsset).
		Select("cmd_ids").
		Where("id IN ?", assetIds).
		Find(&cmdIds).
		Error; err != nil {
		return nil, err
	}

	// Flatten and unique the command IDs
	ids := make([]int, 0)
	for _, s := range cmdIds {
		ids = append(ids, s...)
	}

	return lo.Uniq(ids), nil
}
