package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// GatewayRepository interface for gateway data access
type GatewayRepository interface {
	AttachAssetCount(ctx context.Context, gateways []*model.Gateway) error
	CheckAssetDependencies(ctx context.Context, id int) (string, error)
	BuildQuery(ctx *gin.Context) *gorm.DB
	FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB
}

// gatewayRepository implements GatewayRepository
type gatewayRepository struct{}

// NewGatewayRepository creates a new gateway repository
func NewGatewayRepository() GatewayRepository {
	return &gatewayRepository{}
}

// BuildQuery builds the base query for gateways with filters
func (r *gatewayRepository) BuildQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.Gateway{})

	// Apply filters
	db = r.filterEqual(ctx, db, "id", "type")
	db = r.filterLike(ctx, db, "name")
	db = r.filterSearch(ctx, db, "name", "host", "account", "port")

	// Handle IDs parameter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","),
			func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Sort by name
	db = db.Order("name")

	return db
}

// FilterByAssetIds filters gateways by related asset IDs
func (r *gatewayRepository) FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB {
	if len(assetIds) == 0 {
		return db.Where("0 = 1") // Return empty result if no asset IDs
	}

	subQuery := dbpkg.DB.Model(&model.Asset{}).
		Select("gateway_id").
		Where("id IN ?", assetIds).
		Group("gateway_id")

	return db.Where("id IN (?)", subQuery)
}

// AttachAssetCount attaches asset count to gateways
func (r *gatewayRepository) AttachAssetCount(ctx context.Context, gateways []*model.Gateway) error {
	post := make([]*model.GatewayCount, 0)
	if err := dbpkg.DB.
		Model(&model.Asset{}).
		Select("gateway_id AS id, COUNT(*) AS count").
		Where("gateway_id IN ?", lo.Map(gateways, func(d *model.Gateway, _ int) int { return d.Id })).
		Group("gateway_id").
		Find(&post).
		Error; err != nil {
		return err
	}

	m := lo.SliceToMap(post, func(p *model.GatewayCount) (int, int64) { return p.Id, p.Count })
	for _, d := range gateways {
		d.AssetCount = m[d.Id]
	}
	return nil
}

// CheckAssetDependencies checks if gateway has dependent assets
func (r *gatewayRepository) CheckAssetDependencies(ctx context.Context, id int) (string, error) {
	var assetName string
	err := dbpkg.DB.
		Model(&model.Asset{}).
		Select("name").
		Where("gateway_id = ?", id).
		First(&assetName).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return assetName, errors.New("gateway has dependent assets")
}

// Filter helpers
func (r *gatewayRepository) filterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(fmt.Sprintf("%s = ?", f), q)
		}
	}
	return db
}

func (r *gatewayRepository) filterLike(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
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

func (r *gatewayRepository) filterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
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
