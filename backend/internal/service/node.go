package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/cache"
	"github.com/veops/oneterm/pkg/config"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	kFmtAllNodes = "allNodes"
)

// NodeService handles node business logic
type NodeService struct {
	repo repository.NodeRepository
}

// NewNodeService creates a new node service
func NewNodeService() *NodeService {
	return &NodeService{
		repo: repository.NewNodeRepository(),
	}
}

// ClearCache clears node-related cache
func (s *NodeService) ClearCache(ctx context.Context) error {
	return cache.RC.Del(ctx, kFmtAllNodes).Err()
}

// CheckCycle checks if a node change would create a cycle in the tree
func (s *NodeService) CheckCycle(ctx context.Context, data *model.Node, nodeId int) error {
	nodes := make([]*model.Node, 0)
	err := dbpkg.DB.Model(model.DefaultNode).Find(&nodes).Error
	if err != nil {
		return err
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	var dfs func(int) bool
	dfs = func(x int) bool {
		b := x == data.ParentId
		for _, y := range g[x] {
			b = b || dfs(y)
		}
		return b
	}

	if dfs(nodeId) {
		return errors.New("node change would create cycle")
	}

	return nil
}

// BuildQuery constructs node query with basic filters
func (s *NodeService) BuildQuery(ctx *gin.Context, currentUser interface{}, info bool) (*gorm.DB, error) {
	db := dbpkg.DB.Model(model.DefaultNode)

	// Apply filters
	db = s.filterEqual(ctx, db, "id", "parent_id")
	db = s.filterLike(ctx, db, "name")
	db = s.filterSearch(ctx, db, "name")

	// Handle IDs filter
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	// Handle no_self_child filter
	if id, ok := ctx.GetQuery("no_self_child"); ok {
		ids, err := s.handleNoSelfChild(ctx, cast.ToInt(id))
		if err != nil {
			return nil, err
		}
		db = db.Where("id IN ?", ids)
	}

	// Handle self_parent filter
	if id, ok := ctx.GetQuery("self_parent"); ok {
		ids, err := repository.HandleSelfParent(ctx, cast.ToInt(id))
		if err != nil {
			return nil, err
		}
		db = db.Where("id IN ?", ids)
	}

	// Info mode handling
	if info {
		db = db.Select("id", "parent_id", "name")

		user, ok := currentUser.(acl.Session)
		if ok && !acl.IsAdmin(&user) {
			ids, err := s.GetNodeIdsByAuthorization(ctx)
			if err != nil {
				return nil, err
			}

			ids, err = repository.HandleSelfParent(ctx, ids...)
			if err != nil {
				return nil, err
			}

			db = db.Where("id IN ?", ids)
		}
	}

	return db, nil
}

// AttachAssetCount attaches asset count to nodes
func (s *NodeService) AttachAssetCount(ctx *gin.Context, data []*model.Node) error {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get assets
	assets := make([]*model.AssetIdPid, 0)
	db := dbpkg.DB.Model(model.DefaultAsset)

	if !acl.IsAdmin(currentUser) {
		info := cast.ToBool(ctx.Query("info"))
		if info {
			assetIds, err := s.GetAssetIdsByAuthorization(ctx)
			if err != nil {
				return err
			}
			db = db.Where("id IN ?", assetIds)
		} else {
			assetResId, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
			if err != nil {
				return err
			}
			db, err = s.handleAssetIds(ctx, db, assetResId)
			if err != nil {
				return err
			}
		}
	}

	if err := db.Find(&assets).Error; err != nil {
		logger.L().Error("node posthookfailed asset count", zap.Error(err))
		return err
	}

	// Get nodes
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		logger.L().Error("node posthookfailed node", zap.Error(err))
		return err
	}

	// Calculate asset counts
	m := make(map[int]int64)
	for _, a := range assets {
		m[a.ParentId] += 1
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	var dfs func(int) int64
	dfs = func(x int) int64 {
		for _, y := range g[x] {
			m[x] += dfs(y)
		}
		return m[x]
	}
	dfs(0)

	// Update data with asset counts
	for _, d := range data {
		d.AssetCount = m[d.Id]
	}

	return nil
}

// AttachHasChild attaches hasChild flag to nodes
func (s *NodeService) AttachHasChild(ctx *gin.Context, data []*model.Node) error {
	info := cast.ToBool(ctx.Query("info"))
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return err
	}

	if acl.IsAdmin(currentUser) {
		ps := lo.SliceToMap(nodes, func(n *model.Node) (int, bool) { return n.ParentId, true })
		for _, n := range data {
			n.HasChild = ps[n.Id]
		}
	} else {
		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return err
		}

		if info {
			assetIds, err := s.GetAssetIdsByAuthorization(ctx)
			if err != nil {
				return err
			}

			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })
			pids := lo.Map(assets, func(a *model.Asset, _ int) int { return a.ParentId })
			pids, err = repository.HandleSelfParent(ctx, pids...)
			if err != nil {
				return err
			}

			nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(pids, n.Id) })
			ps := lo.SliceToMap(nodes, func(a *model.Node) (int, bool) { return a.ParentId, true })
			for _, n := range data {
				n.HasChild = ps[n.Id]
			}
		} else {
			var assetResIds, nodeResIds, pids, nids []int
			eg := errgroup.Group{}

			eg.Go(func() (err error) {
				assetResIds, err = acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_ASSET)
				if err != nil {
					return err
				}

				assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetResIds, a.ResourceId) })
				pids = lo.Map(assets, func(n *model.Asset, _ int) int { return n.ParentId })
				pids, err = repository.HandleSelfParent(ctx, pids...)
				return err
			})

			eg.Go(func() (err error) {
				nodeResIds, err = acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
				if err != nil {
					return err
				}

				ns := lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(nodeResIds, n.ResourceId) })
				nids, err = repository.HandleSelfChild(ctx, lo.Map(ns, func(n *model.Node, _ int) int { return n.Id })...)
				if err != nil {
					return err
				}

				nids, err = repository.HandleSelfParent(ctx, nids...)
				return err
			})

			if err := eg.Wait(); err != nil {
				return err
			}

			nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(pids, n.Id) || lo.Contains(nids, n.Id) })
			ps := lo.SliceToMap(nodes, func(n *model.Node) (int, bool) { return n.ParentId, true })
			for _, n := range data {
				n.HasChild = ps[n.Id]
			}
		}
	}

	return nil
}

// CheckDependencies checks if node has dependent assets or child nodes
func (s *NodeService) CheckDependencies(ctx context.Context, id int) (string, error) {
	// Check for child nodes
	var hasChildNode bool
	if err := dbpkg.DB.Model(model.DefaultNode).
		Select("1").
		Where("parent_id = ?", id).
		First(&hasChildNode).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	} else if hasChildNode {
		return "child node", nil
	}

	// Check for assets
	var assetName string
	if err := dbpkg.DB.Model(model.DefaultAsset).
		Select("name").
		Where("parent_id = ?", id).
		First(&assetName).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		return "", nil // No dependencies
	}

	return assetName, nil
}

// GetNodeIdsByAuthorization gets node IDs that the user is authorized to access
func (s *NodeService) GetNodeIdsByAuthorization(ctx *gin.Context) ([]int, error) {
	assetIds, err := s.GetAssetIdsByAuthorization(ctx)
	if err != nil {
		return nil, err
	}

	assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}

	assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })
	ids := lo.Uniq(lo.Map(assets, func(a *model.Asset, _ int) int { return a.ParentId }))

	return ids, nil
}

// GetAssetIdsByAuthorization gets asset IDs that the user is authorized to access
func (s *NodeService) GetAssetIdsByAuthorization(ctx *gin.Context) ([]int, error) {
	// Implementation without controller dependency
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get authorization resource IDs
	resources, err := acl.GetRoleResources(ctx, currentUser.GetRid(), config.RESOURCE_AUTHORIZATION)
	if err != nil {
		return nil, err
	}

	resourceIds := lo.Map(resources, func(r *acl.Resource, _ int) int { return r.ResourceId })

	// Get authorization IDs
	authIds := []*model.AuthorizationIds{}
	if err := dbpkg.DB.Model(&model.AuthorizationIds{}).Where("resource_id IN ?", resourceIds).Find(&authIds).Error; err != nil {
		return nil, err
	}

	// Use asset service to get IDs by authorization IDs
	assetService := NewAssetService()
	_, assetIds, _ := assetService.GetIdsByAuthorizationIds(ctx, authIds)
	return assetIds, nil
}

// HandleNoSelfChild gets all node IDs except self and children
func (s *NodeService) handleNoSelfChild(ctx context.Context, id int) ([]int, error) {
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	ids, err := repository.HandleSelfChild(ctx, id)
	if err != nil {
		return nil, err
	}

	return lo.Filter(lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id }), func(i int, _ int) bool {
		return !lo.Contains(ids, i)
	}), nil
}

// Helper methods for filtering
func (s *NodeService) filterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(f+" = ?", q)
		}
	}
	return db
}

func (s *NodeService) filterLike(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	likes := false
	d := dbpkg.DB
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok && q != "" {
			d = d.Or(f+" LIKE ?", "%"+q+"%")
			likes = true
		}
	}
	if !likes {
		return db
	}
	db = db.Where(d)
	return db
}

func (s *NodeService) filterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	q, ok := ctx.GetQuery("search")
	if !ok || len(fields) <= 0 {
		return db
	}

	d := dbpkg.DB
	for _, f := range fields {
		d = d.Or(f+" LIKE ?", "%"+q+"%")
	}

	db = db.Where(d)
	return db
}

func (s *NodeService) handleAssetIds(ctx context.Context, dbFind *gorm.DB, resIds []int) (*gorm.DB, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	nodeResIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), config.RESOURCE_NODE)
	if err != nil {
		return nil, err
	}

	nodes = lo.Filter(nodes, func(n *model.Node, _ int) bool { return lo.Contains(nodeResIds, n.ResourceId) })
	nodeIds := lo.Map(nodes, func(n *model.Node, _ int) int { return n.Id })

	if nodeIds, err = repository.HandleSelfChild(ctx, nodeIds...); err != nil {
		return nil, err
	}

	d := dbpkg.DB.Where("resource_id IN ?", resIds).Or("parent_id IN?", nodeIds)
	db := dbFind.Where(d)

	return db, nil
}
