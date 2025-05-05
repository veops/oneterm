package repository

import (
	"context"

	"github.com/samber/lo"
	"github.com/veops/oneterm/internal/model"
)

// NodeRepository defines the interface for node repository
type NodeRepository interface {
	// Add required methods
	// These can be empty implementations for now
}

type nodeRepository struct{}

// NewNodeRepository creates a new node repository
func NewNodeRepository() NodeRepository {
	return &nodeRepository{}
}

// HandleSelfChild gets IDs of nodes that are children of the specified node IDs
func HandleSelfChild(ctx context.Context, ids ...int) (res []int, err error) {
	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	var dfs func(int, bool)
	dfs = func(x int, b bool) {
		if b {
			res = append(res, x)
		}
		for _, y := range g[x] {
			dfs(y, b || lo.Contains(ids, x))
		}
	}
	dfs(0, false)

	res = lo.Uniq(append(res, ids...))

	return res, nil
}

// HandleSelfParent gets IDs of nodes that are parents of the specified node IDs
func HandleSelfParent(ctx context.Context, ids ...int) (res []int, err error) {
	nodes, err := GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return nil, err
	}

	g := make(map[int][]int)
	for _, n := range nodes {
		g[n.ParentId] = append(g[n.ParentId], n.Id)
	}

	t := make([]int, 0)
	var dfs func(int)
	dfs = func(x int) {
		t = append(t, x)
		if lo.Contains(ids, x) {
			res = append(res, t...)
		}
		for _, y := range g[x] {
			dfs(y)
		}
		t = t[:len(t)-1]
	}
	dfs(0)

	res = lo.Uniq(append(res, ids...))

	return res, nil
}
