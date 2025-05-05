package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/config"
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
	db = s.filterEqual(ctx, db, "id", "enable")
	db = s.filterLike(ctx, db, "name")
	db = s.filterSearch(ctx, db, "name", "cmd")

	// Handle IDs filter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	return db, nil
}

// GetAuthorizedCommandIds gets command IDs that the user is authorized to access
func (s *CommandService) GetAuthorizedCommandIds(ctx context.Context, currentUser interface{}) ([]int, error) {
	user, ok := currentUser.(acl.Session)
	if !ok {
		return nil, fmt.Errorf("invalid user type")
	}

	rs, err := acl.GetRoleResources(ctx, user.GetRid(), config.RESOURCE_AUTHORIZATION)
	if err != nil {
		return nil, err
	}

	// Get asset IDs from authorization
	sub := dbpkg.DB.
		Model(&model.Authorization{}).
		Select("DISTINCT asset_id").
		Where("resource_id IN ?", lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId }))

	// Get command IDs from assets
	cmdIds := make([]model.Slice[int], 0)
	if err = dbpkg.DB.
		Model(model.DefaultAsset).
		Select("cmd_ids").
		Where("id IN (?)", sub).
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

// Helper methods for filtering
func (s *CommandService) filterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(fmt.Sprintf("%s = ?", f), q)
		}
	}
	return db
}

func (s *CommandService) filterLike(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	likes := false
	d := dbpkg.DB
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok && q != "" {
			d = d.Or(fmt.Sprintf("%s LIKE ?", f), fmt.Sprintf("%%%s%%", q))
			likes = true
		}
	}
	if !likes {
		return db
	}
	db = db.Where(d)
	return db
}

func (s *CommandService) filterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	q, ok := ctx.GetQuery("search")
	if !ok || len(fields) <= 0 {
		return db
	}

	d := dbpkg.DB
	for _, f := range fields {
		d = d.Or(fmt.Sprintf("%s LIKE ?", f), fmt.Sprintf("%%%s%%", q))
	}

	db = db.Where(d)
	return db
}
