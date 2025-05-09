package repository

import (
	"context"
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

const (
	kFmtAssetIds      = "assetIds-%d"
	kAuthorizationIds = "authorizationIds"
	kNodeIds          = "nodeIds"
	kAccountIds       = "accountIds"
)

// AssetRepository interface for asset data access
type AssetRepository interface {
	GetById(ctx context.Context, id int) (*model.Asset, error)
	AttachNodeChain(ctx context.Context, assets []*model.Asset) error
	ApplyAuthorizationFilters(ctx *gin.Context, assets []*model.Asset, authorizationIds []*model.AuthorizationIds, nodeIds, accountIds []int)
	BuildQuery(ctx *gin.Context) (*gorm.DB, error)
	FilterByParentId(db *gorm.DB, parentId int) (*gorm.DB, error)
	GetAssetIdsByAuthorization(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) ([]int, []int, []int, error)
	GetIdsByAuthorizationIds(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) ([]int, []int, []int)
	GetAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) ([]int, error)
}

// assetRepository implements AssetRepository
type assetRepository struct{}

// NewAssetRepository creates a new asset repository
func NewAssetRepository() AssetRepository {
	return &assetRepository{}
}

// GetById retrieves an asset by its ID
func (r *assetRepository) GetById(ctx context.Context, id int) (*model.Asset, error) {
	asset := &model.Asset{}
	err := dbpkg.DB.Where("id = ?", id).First(asset).Error
	return asset, err
}

// BuildQuery builds the base query for assets with filters
func (r *assetRepository) BuildQuery(ctx *gin.Context) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultAsset)

	// Apply filters
	db = dbpkg.FilterEqual(ctx, db, "id")
	db = dbpkg.FilterLike(ctx, db, "name", "ip")
	db = dbpkg.FilterSearch(ctx, db, "name", "ip")

	// Handle IDs parameter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","),
			func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Sort by name
	db = db.Order("name")

	return db, nil
}

// FilterByParentId filters assets by parent ID and its children
func (r *assetRepository) FilterByParentId(db *gorm.DB, parentId int) (*gorm.DB, error) {
	parentIds, err := r.handleParentId(context.Background(), parentId)
	if err != nil {
		return nil, err
	}
	return db.Where("parent_id IN ?", parentIds), nil
}

// AttachNodeChain attaches node chain to assets
func (r *assetRepository) AttachNodeChain(ctx context.Context, assets []*model.Asset) error {
	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return err
	}

	g := make(map[int][]model.Pair[int, string])
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], model.Pair[int, string]{First: n.Id, Second: n.Name})
	}

	m := make(map[int]string)
	var dfs func(int, string)
	dfs = func(x int, s string) {
		m[x] = s
		for _, node := range g[x] {
			dfs(node.First, fmt.Sprintf("%s/%s", s, node.Second))
		}
	}
	dfs(0, "")

	for _, d := range assets {
		d.NodeChain = m[d.ParentId]
	}

	return nil
}

// ApplyAuthorizationFilters applies authorization filters to assets
func (r *assetRepository) ApplyAuthorizationFilters(ctx *gin.Context, assets []*model.Asset, authorizationIds []*model.AuthorizationIds, nodeIds, accountIds []int) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if acl.IsAdmin(currentUser) {
		return
	}

	info := cast.ToBool(ctx.Query("info"))
	noInfoIds := make([]int, 0)

	if !info {
		t := dbpkg.DB.Model(model.DefaultAsset)
		assetResIds, _ := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
		t, _ = r.handleAssetIds(ctx, t, assetResIds)
		t.Pluck("id", &noInfoIds)
	}

	for _, a := range assets {
		if lo.Contains(nodeIds, a.ParentId) || lo.Contains(noInfoIds, a.Id) {
			continue
		}
		if lo.ContainsBy(authorizationIds, func(item *model.AuthorizationIds) bool {
			return item.AssetId == a.Id && item.NodeId == 0 && item.AccountId == 0
		}) {
			continue
		}
		ids := lo.Map(lo.Filter(authorizationIds, func(item *model.AuthorizationIds, _ int) bool {
			return item.AssetId == a.Id && item.AccountId != 0 && item.NodeId == 0
		}),
			func(item *model.AuthorizationIds, _ int) int { return item.AccountId })

		for k := range a.Authorization {
			if !lo.Contains(ids, k) && !lo.Contains(accountIds, k) {
				delete(a.Authorization, k)
			}
		}
	}
}

// GetAssetIdsByAuthorization gets asset IDs by authorization
func (r *assetRepository) GetAssetIdsByAuthorization(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) ([]int, []int, []int, error) {
	ctx.Set(kAuthorizationIds, authorizationIds)

	nodeIds, assetIds, accountIds := r.GetIdsByAuthorizationIds(ctx, authorizationIds)

	tmp, err := HandleSelfChild(ctx, nodeIds...)
	if err != nil {
		return nil, nil, nil, err
	}

	nodeIds = append(nodeIds, tmp...)
	ctx.Set(kNodeIds, nodeIds)
	ctx.Set(kAccountIds, accountIds)

	assetIdsFromNode, err := r.GetAssetIdsByNodeAccount(ctx, nodeIds, accountIds)
	if err != nil {
		return nil, nil, nil, err
	}

	allAssetIds := lo.Uniq(append(assetIds, assetIdsFromNode...))

	return nodeIds, allAssetIds, accountIds, nil
}

// GetIdsByAuthorizationIds extracts node IDs, asset IDs, and account IDs from authorization IDs
func (r *assetRepository) GetIdsByAuthorizationIds(ctx *gin.Context, authorizationIds []*model.AuthorizationIds) (nodeIds, assetIds, accountIds []int) {
	info := cast.ToBool(ctx.Query("info"))
	for _, a := range authorizationIds {
		if a.NodeId != 0 && a.AssetId == 0 && a.AccountId == 0 {
			nodeIds = append(nodeIds, a.NodeId)
		}
		if a.AssetId != 0 && a.NodeId == 0 && (info || a.AccountId == 0) {
			assetIds = append(assetIds, a.AssetId)
		}
		if a.AccountId != 0 && a.AssetId == 0 && (info || a.NodeId == 0) {
			accountIds = append(accountIds, a.AccountId)
		}
	}
	return
}

// GetAssetIdsByNodeAccount gets asset IDs by node IDs and account IDs
func (r *assetRepository) GetAssetIdsByNodeAccount(ctx context.Context, nodeIds, accountIds []int) (assetIds []int, err error) {
	assets, err := GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}

	assets = lo.Filter(assets, func(a *model.Asset, _ int) bool {
		return lo.Contains(nodeIds, a.ParentId) || len(lo.Intersect(lo.Keys(a.Authorization), accountIds)) > 0
	})

	assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })
	return assetIds, nil
}

// handleParentId builds a slice of parent IDs including the given parent ID and all its children
func (r *assetRepository) handleParentId(ctx context.Context, parentId int) (pids []int, err error) {
	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	var dfs func(int)
	dfs = func(x int) {
		pids = append(pids, x)
		for _, y := range g[x] {
			dfs(y)
		}
	}
	dfs(parentId)

	return pids, nil
}

// handleAssetIds filters assets by resource IDs and node IDs
func (r *assetRepository) handleAssetIds(ctx *gin.Context, dbFind *gorm.DB, resIds []int) (db *gorm.DB, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	nodeResIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
	if err != nil {
		return nil, err
	}

	nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(nodeResIds, n.ResourceId) })
	nodeIds := lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })

	if nodeIds, err = HandleSelfChild(ctx, nodeIds...); err != nil {
		return nil, err
	}

	d := dbpkg.DB.Where("resource_id IN ?", resIds).Or("parent_id IN?", nodeIds)
	db = dbFind.Where(d)

	return db, nil
}

// HandleAssetIds filters asset queries based on resource IDs
func HandleAssetIds(ctx context.Context, dbFind *gorm.DB, resIds []int) (db *gorm.DB, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}
	nodeResIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
	if err != nil {
		return
	}
	nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(nodeResIds, n.ResourceId) })
	nodeIds := lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })
	if nodeIds, err = HandleSelfChild(ctx, nodeIds...); err != nil {
		return
	}

	d := dbpkg.DB.Where("resource_id IN ?", resIds).Or("parent_id IN?", nodeIds)

	db = dbFind.Where(d)

	return
}
