package controller

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/errors"
)

var (
	nodeService = service.NewNodeService()

	nodePreHooks  = []preHook[*model.Node]{nodePreHookCheckCycle}
	nodePostHooks = []postHook[*model.Node]{nodePostHookCountAsset, nodePostHookHasChild}
	nodeDcs       = []deleteCheck{nodeDelHook}
)

// CreateNode godoc
//
//	@Tags		node
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node [post]
func (c *Controller) CreateNode(ctx *gin.Context) {
	if err := nodeService.ClearCache(ctx); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	doCreate(ctx, true, &model.Node{}, config.RESOURCE_NODE)
}

// DeleteNode godoc
//
//	@Tags		node
//	@Param		id	path		int	true	"node id"
//	@Success	200	{object}	HttpResponse
//	@Router		/node/:id [delete]
func (c *Controller) DeleteNode(ctx *gin.Context) {
	if err := nodeService.ClearCache(ctx); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	doDelete(ctx, true, &model.Node{}, config.RESOURCE_NODE, nodeDcs...)
}

// UpdateNode godoc
//
//	@Tags		node
//	@Param		id		path		int			true	"node id"
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node/:id [put]
func (c *Controller) UpdateNode(ctx *gin.Context) {
	if err := nodeService.ClearCache(ctx); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	doUpdate(ctx, true, &model.Node{}, config.RESOURCE_NODE, nodePreHooks...)
}

// GetNodes godoc
//
//	@Tags		node
//	@Param		page_index		query		int		true	"node id"
//	@Param		page_size		query		int		true	"node id"
//	@Param		id				query		int		false	"node id"
//	@Param		ids				query		string	false	"node ids"
//	@Param		parent_id		query		int		false	"node's parent id"
//	@Param		name			query		string	false	"node name"
//	@Param		no_self_child	query		int		false	"exclude itself and its child"
//	@Param		self_parent		query		int		false	"include itself and its parent"
//	@Param		recursive		query		bool	false	"return tree structure with children"
//	@Success	200				{object}	HttpResponse{data=ListData{list=[]model.Node}}
//	@Router		/node [get]
func (c *Controller) GetNodes(ctx *gin.Context) {
	info := cast.ToBool(ctx.Query("info"))
	recursive := cast.ToBool(ctx.Query("recursive"))

	// Build query with integrated V2 authorization filter
	db, err := nodeService.BuildQueryWithAuthorization(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply info mode settings
	if info {
		db = db.Select("id", "parent_id", "name", "authorization")
	}

	if recursive {
		treeNodes, err := nodeService.GetNodesTree(ctx, db, !info, config.RESOURCE_NODE)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}

		res := &ListData{
			Count: int64(len(treeNodes)),
			List:  treeNodes,
		}

		ctx.JSON(http.StatusOK, NewHttpResponseWithData(res))
	} else {
		doGet(ctx, !info, db, config.RESOURCE_NODE, nodePostHooks...)
	}
}

func nodePreHookCheckCycle(ctx *gin.Context, data *model.Node) {
	if err := nodeService.CheckCycle(ctx, data, cast.ToInt(ctx.Param("id"))); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument})
	}
}

func nodePostHookCountAsset(ctx *gin.Context, data []*model.Node) {
	if err := nodeService.AttachAssetCount(ctx, data); err != nil {
		// Just log error, don't abort
	}
}

func nodePostHookHasChild(ctx *gin.Context, data []*model.Node) {
	if err := nodeService.AttachHasChild(ctx, data); err != nil {
		// Just log error, don't abort
	}
}

func nodeDelHook(ctx *gin.Context, id int) {
	assetName, err := nodeService.CheckDependencies(ctx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	if assetName != "" {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrHasDepency, Data: map[string]any{"name": assetName}})
	}
}

func GetNodeIdsByAuthorization(ctx *gin.Context) (ids []int, err error) {
	return nodeService.GetNodeIdsByAuthorization(ctx)
}

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

func getNodeId2ResId(ctx context.Context) (resid2ids map[int]int, err error) {
	nodes, err := repository.GetAllFromCacheDb(ctx, model.DefaultNode)
	if err != nil {
		return
	}

	resid2ids = lo.SliceToMap(nodes, func(n *model.Node) (int, int) { return n.Id, n.ResourceId })

	return
}
