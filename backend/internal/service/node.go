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

	db = dbpkg.FilterEqual(ctx, db, "parent_id", "id")
	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "id")

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

// BuildQueryWithAuthorization builds query with integrated V2 authorization filter
func (s *NodeService) BuildQueryWithAuthorization(ctx *gin.Context) (*gorm.DB, error) {
	// Start with base query filters (without authorization)
	db := dbpkg.DB.Model(model.DefaultNode)

	db = dbpkg.FilterEqual(ctx, db, "parent_id", "id")
	db = dbpkg.FilterLike(ctx, db, "name")
	db = dbpkg.FilterSearch(ctx, db, "name", "id")

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

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Administrators have access to all nodes
	if acl.IsAdmin(currentUser) {
		return db, nil
	}

	// Apply V2 authorization filter: get authorized asset IDs using V2 system
	authV2Service := NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
	if err != nil {
		return nil, err
	}

	// Use the same logic as GetNodeIdsByAuthorization but more efficiently
	if len(assetIds) == 0 {
		// No access to any assets = no access to any nodes
		db = db.Where("1 = 0")
	} else {
		// Get parent node IDs from authorized assets (same logic as before)
		var parentIds []int
		err = dbpkg.DB.Model(model.DefaultAsset).
			Where("id IN ?", assetIds).
			Distinct("parent_id").
			Pluck("parent_id", &parentIds).Error
		if err != nil {
			return nil, err
		}

		if len(parentIds) == 0 {
			db = db.Where("1 = 0")
		} else {
			// Include self and parent hierarchy for proper tree navigation
			allNodeIds, err := repository.HandleSelfParent(ctx, parentIds...)
			if err != nil {
				return nil, err
			}
			db = db.Where("id IN ?", allNodeIds)
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
			// Use V2 authorization system for asset filtering
			authV2Service := NewAuthorizationV2Service()
			_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
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
			// Use V2 authorization system for asset filtering
			authV2Service := NewAuthorizationV2Service()
			_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
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
	// Use V2 authorization system for asset filtering
	authV2Service := NewAuthorizationV2Service()
	_, assetIds, _, err := authV2Service.GetAuthorizationScopeByACL(ctx)
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

// GetNodesTree gets node tree with its children
func (s *NodeService) GetNodesTree(ctx *gin.Context, dbQuery *gorm.DB, needAcl bool, resourceType string) ([]any, error) {
	// Get info parameter
	info := cast.ToBool(ctx.Query("info"))

	// 1. First get all nodes that meet the conditions (using the same permission control)
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	db := dbQuery
	if needAcl && !acl.IsAdmin(currentUser) {
		resIds, err := acl.GetRoleResourceIds(ctx, currentUser.GetRid(), resourceType)
		if err != nil {
			return nil, err
		}

		var err2 error
		if db, err2 = repository.HandleNodeIds(ctx, db, resIds); err2 != nil {
			return nil, err2
		}
	}

	// Query filtered nodes (without pagination)
	filteredNodes := make([]*model.Node, 0)
	if err := db.Order("id DESC").Find(&filteredNodes).Error; err != nil {
		return nil, err
	}

	// Extract node IDs from filtered nodes
	nodeIds := make([]int, 0, len(filteredNodes))
	for _, node := range filteredNodes {
		nodeIds = append(nodeIds, node.Id)
	}

	// Get all children of these nodes recursively
	allNodeIds := nodeIds
	childIds, err := repository.HandleSelfChild(ctx, nodeIds...)
	if err != nil {
		logger.L().Error("failed to get child nodes", zap.Error(err))
	} else {
		allNodeIds = childIds
	}

	// Now get all nodes (filtered nodes + their children)
	allNodes := make([]*model.Node, 0)
	query := dbpkg.DB.Model(model.DefaultNode).Where("id IN ?", allNodeIds)

	// If info=true, select only id, parent_id, name (same as in BuildQuery)
	if info {
		query = query.Select("id", "parent_id", "name")
	}

	if err := query.Find(&allNodes).Error; err != nil {
		return nil, err
	}

	// Apply postHooks - now always apply permission handling regardless of info mode
	if err := s.AttachAssetCount(ctx, allNodes); err != nil {
		logger.L().Error("failed to attach asset count", zap.Error(err))
	}

	if err := s.AttachHasChild(ctx, allNodes); err != nil {
		logger.L().Error("failed to attach has_child flag", zap.Error(err))
	}

	// Always handle permissions, regardless of info mode
	if err := s.handleNodePermissions(ctx, allNodes, resourceType); err != nil {
		return nil, err
	}

	// Build tree structure
	return s.buildNodeTree(allNodes, nodeIds), nil
}

// buildNodeTree builds a tree structure from nodes
func (s *NodeService) buildNodeTree(nodes []*model.Node, rootIds []int) []any {
	logger.L().Info("buildNodeTree", zap.Any("nodes", nodes))
	// Node mapping
	nodeMap := make(map[int]*model.Node)
	for _, node := range nodes {
		// Initialize Children for all nodes
		node.Children = make([]*model.Node, 0)
		nodeMap[node.Id] = node
	}

	// First pass: build parent-child relationships for all nodes
	for _, node := range nodes {
		if parent, exists := nodeMap[node.ParentId]; exists && node.ParentId != 0 {
			// Add this node to its parent's children
			parent.Children = append(parent.Children, node)
		}
	}

	// Update has_child flag based on children array
	for _, node := range nodes {
		node.HasChild = len(node.Children) > 0
	}

	// Second pass: collect only specified root nodes
	treeNodes := make([]any, 0)
	rootNodesSet := make(map[int]bool)
	for _, id := range rootIds {
		rootNodesSet[id] = true
	}

	for _, node := range nodes {
		// A node is a root if it's in the rootIds list
		if rootNodesSet[node.Id] {
			treeNodes = append(treeNodes, node)
		}
	}

	return treeNodes
}

// handleNodePermissions handles node permissions
func (s *NodeService) handleNodePermissions(ctx *gin.Context, nodes []*model.Node, resourceType string) error {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	if !lo.Contains(config.PermResource, resourceType) {
		return nil
	}

	res, err := acl.GetRoleResources(ctx, currentUser.GetRid(), resourceType)
	if err != nil {
		return err
	}

	resId2perms := lo.SliceToMap(res, func(r *acl.Resource) (int, []string) {
		return r.ResourceId, r.Permissions
	})

	// Process permission inheritance
	resId2perms, err = handleSelfChildPerms(ctx, resId2perms)
	if err != nil {
		return err
	}

	// Set permissions
	isAdmin := acl.IsAdmin(currentUser)
	for _, node := range nodes {
		if isAdmin {
			node.SetPerms(acl.AllPermissions)
		} else {
			node.SetPerms(resId2perms[node.GetResourceId()])
		}
	}

	return nil
}

// handleSelfChildPerms handles permission inheritance (from parent to child nodes)
func handleSelfChildPerms(ctx context.Context, id2perms map[int][]string) (res map[int][]string, err error) {
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	res = make(map[int][]string)
	id2rid := make(map[int]int)
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
		id2rid[n.Id] = n.ResourceId
		res[id2rid[n.Id]] = id2perms[id2rid[n.Id]]
	}
	var dfs func(int)
	dfs = func(x int) {
		for _, y := range g[x] {
			res[id2rid[y]] = lo.Uniq(append(res[id2rid[y]], res[id2rid[x]]...))
			dfs(y)
		}
	}
	dfs(0)

	return
}
