package repository

import (
	"context"
	"errors"
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
	db = dbpkg.FilterEqual(ctx, db, "id", "type")
	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "host", "account", "port")

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
