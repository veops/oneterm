package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/pkg/cache"
)

// StatService handles statistics business logic
type StatService struct {
	repo repository.StatRepository
}

// NewStatService creates a new stat service
func NewStatService() *StatService {
	return &StatService{
		repo: repository.NewStatRepository(),
	}
}

// GetAssetTypes gets asset type statistics
func (s *StatService) GetAssetTypes(ctx context.Context) ([]*model.StatAssetType, error) {
	// Try to get from cache first
	stat := make([]*model.StatAssetType, 0)
	key := "stat-assettype"
	if cache.Get(ctx, key, stat) == nil {
		return stat, nil
	}

	// Get node-asset counts
	m, err := s.nodeCountAsset(ctx)
	if err != nil {
		return nil, err
	}

	// Get stat data
	stat, err = s.repo.GetStatNodeAssetTypes(ctx)
	if err != nil {
		return nil, err
	}

	// Set counts
	for _, s := range stat {
		s.Count = m[s.Id]
	}

	// Save to cache
	cache.SetEx(ctx, key, stat, time.Minute)

	return stat, nil
}

// GetStatCount gets overall statistics
func (s *StatService) GetStatCount(ctx context.Context) (*model.StatCount, error) {
	// Try to get from cache first
	stat := &model.StatCount{}
	key := "stat-count"
	if cache.Get(ctx, key, stat) == nil {
		return stat, nil
	}

	// Get stat data
	stat, err := s.repo.GetStatCount(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure gateway count doesn't exceed total
	stat.Gateway = lo.Ternary(stat.Gateway <= stat.TotalGateway, stat.Gateway, stat.TotalGateway)

	// Save to cache
	cache.SetEx(ctx, key, stat, time.Minute)

	return stat, nil
}

// GetStatAccount gets account usage statistics
func (s *StatService) GetStatAccount(ctx context.Context, timeRange string) ([]*model.StatAccount, error) {
	// Calculate time range
	start, end := time.Now(), time.Now()
	switch timeRange {
	case "day":
		start = start.Add(-time.Hour * 24)
	case "week":
		start = start.Add(-time.Hour * 24 * 7)
	case "month":
		start = start.Add(-time.Hour * 24 * 30)
	default:
		return nil, fmt.Errorf("wrong time range %s", timeRange)
	}

	// Try to get from cache first
	stat := make([]*model.StatAccount, 0)
	key := "stat-account-" + timeRange
	if cache.Get(ctx, key, stat) == nil {
		return stat, nil
	}

	// Get stat data
	stat, err := s.repo.GetStatAccount(ctx, start, end)
	if err != nil {
		return nil, err
	}

	// Save to cache
	cache.SetEx(ctx, key, stat, time.Minute)

	return stat, nil
}

// GetStatAsset gets asset usage statistics
func (s *StatService) GetStatAsset(ctx context.Context, timeRange string) ([]*model.StatAsset, error) {
	// Set parameters based on time range
	start, end := time.Now(), time.Now()
	interval := time.Hour * 24
	dateFmt := "%Y-%m-%d"
	timeFmt := time.DateOnly

	switch timeRange {
	case "day":
		start = start.Add(-time.Hour * 24)
		interval = time.Hour
		dateFmt = "%Y-%m-%d %H:00:00"
		timeFmt = time.DateTime
	case "week":
		start = start.Add(-time.Hour * 24 * 7)
	case "month":
		start = start.Add(-time.Hour * 24 * 30)
	default:
		return nil, fmt.Errorf("wrong time range %s", timeRange)
	}

	// Try to get from cache first
	stat := make([]*model.StatAsset, 0)
	key := "stat-asset-" + timeRange
	if cache.Get(ctx, key, stat) == nil {
		return stat, nil
	}

	// Get stat data
	stat, err := s.repo.GetStatAsset(ctx, start, end, dateFmt)
	if err != nil {
		return nil, err
	}

	// Fill in missing time slots
	for t := start; !t.After(end); t = t.Add(interval) {
		timeStr := t.Truncate(interval).Format(timeFmt)
		if lo.ContainsBy(stat, func(s *model.StatAsset) bool { return timeStr == s.Time }) {
			continue
		}
		stat = append(stat, &model.StatAsset{Time: timeStr})
	}

	// Sort by time
	sort.Slice(stat, func(i, j int) bool { return stat[i].Time < stat[j].Time })

	// Save to cache
	cache.SetEx(ctx, key, stat, time.Minute)

	return stat, nil
}

// GetStatCountOfUser gets statistics for current user
func (s *StatService) GetStatCountOfUser(ctx *gin.Context) (*model.StatCountOfUser, error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get accessible assets for non-admin users
	var assetIds []int
	if !acl.IsAdmin(currentUser) {
		// Get assets through acl permissions
		_, ids, _, err := getNodeAssetAccoutIdsByAction(ctx, acl.READ)
		if err != nil {
			return nil, err
		}
		assetIds = ids
	} else {
		// For admin, get all assets
		assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
		if err != nil {
			return nil, err
		}
		assetIds = lo.Map(assets, func(a *model.Asset, _ int) int { return a.Id })
	}

	// Get stats
	return s.repo.GetStatCountOfUser(ctx, currentUser.GetUid(), assetIds)
}

// GetStatRankOfUser gets user ranking statistics
func (s *StatService) GetStatRankOfUser(ctx context.Context, limit int) ([]*model.StatRankOfUser, error) {
	return s.repo.GetStatRankOfUser(ctx, limit)
}

// Helper methods

// nodeCountAsset counts assets for each node
func (s *StatService) nodeCountAsset(ctx context.Context) (map[int]int64, error) {
	// Get all nodes and assets
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	assets, err := repository.GetAllFromCacheDb(ctx, model.DefaultAsset)
	if err != nil {
		return nil, err
	}

	// Build node tree
	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	// Count assets per node
	m := make(map[int]int64)
	for _, a := range assets {
		m[a.ParentId]++
	}

	// Calculate total (including children) for each node
	var dfs func(int) int64
	dfs = func(x int) int64 {
		for _, y := range g[x] {
			m[x] += dfs(y)
		}
		return m[x]
	}
	dfs(0)

	return m, nil
}
