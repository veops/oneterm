package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"gorm.io/gorm"
)

// AccountRepository interface for account data access
type AccountRepository interface {
	AttachAssetCount(ctx context.Context, accounts []*model.Account) error
	CheckAssetDependencies(ctx context.Context, id int) (string, error)
	BuildQuery(ctx *gin.Context) *gorm.DB
	FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB
	GetAccountIdsByAuthorization(ctx context.Context, assetIds []int, authorizationIds []int) ([]int, error)
}

// accountRepository implements AccountRepository
type accountRepository struct{}

// NewAccountRepository creates a new account repository
func NewAccountRepository() AccountRepository {
	return &accountRepository{}
}

// BuildQuery builds the base query for accounts with filters
func (r *accountRepository) BuildQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.Account{})

	// Apply filters
	db = r.filterEqual(ctx, db, "id", "type")
	db = r.filterLike(ctx, db, "name")
	db = r.filterSearch(ctx, db, "name", "account")

	// Handle IDs parameter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","),
			func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Sort by name
	db = db.Order("name")

	return db
}

// FilterByAssetIds filters accounts by related asset IDs
func (r *accountRepository) FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB {
	if len(assetIds) == 0 {
		return db.Where("0 = 1") // Return empty result if no asset IDs
	}

	// Query account IDs associated with specified assets
	subQuery := dbpkg.DB.Model(&model.Authorization{}).
		Select("account_id").
		Where("asset_id IN ?", assetIds).
		Group("account_id")

	return db.Where("id IN (?)", subQuery)
}

// AttachAssetCount attaches asset count to accounts
func (r *accountRepository) AttachAssetCount(ctx context.Context, accounts []*model.Account) error {
	acs := make([]*model.AccountCount, 0)
	if err := dbpkg.DB.
		Model(&model.Authorization{}).
		Select("account_id AS id, COUNT(*) as count").
		Group("account_id").
		Where("account_id IN ?", lo.Map(accounts, func(d *model.Account, _ int) int { return d.Id })).
		Find(&acs).
		Error; err != nil {
		return err
	}

	m := lo.SliceToMap(acs, func(ac *model.AccountCount) (int, int64) { return ac.Id, ac.Count })
	for _, d := range accounts {
		d.AssetCount = m[d.Id]
	}
	return nil
}

// CheckAssetDependencies checks if account has dependent assets
func (r *accountRepository) CheckAssetDependencies(ctx context.Context, id int) (string, error) {
	var assetName string
	err := dbpkg.DB.
		Model(model.DefaultAsset).
		Select("name").
		Where("id = (?)", dbpkg.DB.Model(&model.Authorization{}).Select("asset_id").Where("account_id = ?", id).Limit(1)).
		First(&assetName).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return assetName, errors.New("account has dependent assets")
}

// GetAccountIdsByAuthorization gets account IDs by authorization and asset IDs
func (r *accountRepository) GetAccountIdsByAuthorization(ctx context.Context, assetIds []int, authorizationIds []int) ([]int, error) {
	// Get account IDs from assets' authorization lists
	ss := make([]model.Slice[string], 0)
	if err := dbpkg.DB.Model(model.DefaultAsset).Where("id IN ?", assetIds).Pluck("JSON_KEYS(authorization)", &ss).Error; err != nil {
		return nil, err
	}

	// Process account IDs obtained from assets
	accountIds := lo.Uniq(lo.Map(lo.Flatten(ss), func(s string, _ int) int { return cast.ToInt(s) }))

	// Merge with account IDs from authorizations
	return lo.Uniq(append(accountIds, authorizationIds...)), nil
}

// Filter helpers
func (r *accountRepository) filterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(fmt.Sprintf("%s = ?", f), q)
		}
	}
	return db
}

func (r *accountRepository) filterLike(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
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

func (r *accountRepository) filterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
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

// HandleAccountIds filters account queries based on resource IDs
func HandleAccountIds(ctx context.Context, dbFind *gorm.DB, resIds []int) (db *gorm.DB, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	assetResIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
	if err != nil {
		return
	}
	t, _ := HandleAssetIds(ctx, dbpkg.DB.Model(model.DefaultAsset), assetResIds)
	ss := make([]model.Slice[string], 0)
	if err = t.Pluck("JSON_KEYS(authorization)", &ss).Error; err != nil {
		return
	}
	ids := lo.Uniq(lo.Map(lo.Flatten(ss), func(s string, _ int) int { return cast.ToInt(s) }))

	d := dbpkg.DB.Where("resource_id IN ?", resIds).Or("id IN ?", ids)

	db = dbFind.Where(d)

	return
}
