package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
)

var (
	nodePreHooks = []preHook[*model.Node]{
		func(ctx *gin.Context, data *model.Node) {
			ids := make([]int, 0)
			if err := mysql.DB.Raw(fmt.Sprintf(`
				WITH RECURSIVE cte AS(
					SELECT id
					FROM node
					WHERE id=%s AND deleted_at = 0
					UNION ALL
					SELECT t.id
					FROM cte
						INNER JOIN node t on cte.id = t.parent_id
					WHERE deleted_at = 0
				)
				SELECT
					id
				FROM cte
				`, ctx.Param("id"))).
				Find(&ids).
				Error; err != nil || lo.Contains(ids, data.ParentId) {
				ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument})
			}
		},
	}
	nodePostHooks = []postHook[*model.Node]{
		func(ctx *gin.Context, data []*model.Node) {
			currentUser, _ := acl.GetSessionFromCtx(ctx)
			isAdmin := acl.IsAdmin(currentUser)
			post := make([]*model.NodeCount, 0)
			sql := fmt.Sprintf(`
				WITH RECURSIVE cte AS(
					SELECT parent_id
					FROM asset
					%s
					UNION ALL
					SELECT t.parent_id
					FROM cte
						INNER JOIN node t on cte.parent_id = t.id
					WHERE deleted_at = 0
				)
				SELECT
					parent_id,
					COUNT(*) AS count
				FROM cte
				GROUP BY parent_id
			`, lo.Ternary(isAdmin, "WHERE deleted_at = 0", "WHERE deleted_at = 0 AND id IN (?)"))
			db := mysql.DB.
				Model(&model.Asset{})
			if isAdmin {
				db = db.Raw(sql)
			} else {
				authorizationResourceIds, err := GetAutorizationResourceIds(ctx)
				if err != nil {
					ctx.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				db = db.Raw(sql, mysql.DB.Model(&model.Authorization{}).Select("asset_id").Where("resource_id IN ?", authorizationResourceIds))
			}
			if err := db.
				Find(&post).
				Error; err != nil {
				logger.L.Error("node posthookfailed asset count", zap.Error(err))
				return
			}
			m := lo.SliceToMap(post, func(p *model.NodeCount) (int, int64) { return p.ParentId, p.Count })
			for _, d := range data {
				d.AssetCount = m[d.Id]
			}
		}, func(ctx *gin.Context, data []*model.Node) {
			ps := make([]int, 0)
			if err := mysql.DB.
				Model(&model.Node{}).
				Where("parent_id IN ?", lo.Map(data, func(n *model.Node, _ int) int { return n.Id })).
				Pluck("parent_id", &ps).
				Error; err != nil {
				logger.L.Error("node posthookfailed has child", zap.Error(err))
				return
			}
			pm := lo.SliceToMap(ps, func(pid int) (int, bool) { return pid, true })
			for _, n := range data {
				n.HasChild = pm[n.Id]
			}
		},
	}
	nodeDcs = []deleteCheck{
		func(ctx *gin.Context, id int) {
			noChild := true
			noChild = noChild && errors.Is(mysql.DB.Model(&model.Node{}).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
			noChild = noChild && errors.Is(mysql.DB.Model(&model.Asset{}).Select("id").Where("parent_id = ?", id).First(map[string]any{}).Error, gorm.ErrRecordNotFound)
			if noChild {
				return
			}

			err := &ApiError{Code: ErrHasChild, Data: nil}
			ctx.AbortWithError(http.StatusBadRequest, err)
		},
	}
)

// CreateNode godoc
//
//	@Tags		node
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node [post]
func (c *Controller) CreateNode(ctx *gin.Context) {
	doCreate(ctx, false, &model.Node{}, "")
}

// DeleteNode godoc
//
//	@Tags		node
//	@Param		id	path		int	true	"node id"
//	@Success	200	{object}	HttpResponse
//	@Router		/node/:id [delete]
func (c *Controller) DeleteNode(ctx *gin.Context) {
	doDelete(ctx, false, &model.Node{}, nodeDcs...)
}

// UpdateNode godoc
//
//	@Tags		node
//	@Param		id		path		int			true	"node id"
//	@Param		node	body		model.Node	true	"node"
//	@Success	200		{object}	HttpResponse
//	@Router		/node/:id [put]
func (c *Controller) UpdateNode(ctx *gin.Context) {
	doUpdate(ctx, false, &model.Node{}, nodePreHooks...)
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
//	@Success	200				{object}	HttpResponse{data=ListData{list=[]model.Node}}
//	@Router		/node [get]
func (c *Controller) GetNodes(ctx *gin.Context) {
	db := mysql.DB.Model(&model.Node{})

	db = filterEqual(ctx, db, "id", "parent_id")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}
	if id, ok := ctx.GetQuery("no_self_child"); ok {
		sql := fmt.Sprintf(`
		WITH RECURSIVE cte AS(
			SELECT id
			FROM node
			WHERE id=%s AND deleted_at = 0
			UNION ALL
			SELECT t.id
			FROM cte
				INNER JOIN node t on cte.id = t.parent_id
			WHERE deleted_at = 0
		)
		SELECT
			id
		FROM cte
		`, id)
		sub := mysql.DB.Raw(sql)
		db = db.Where("id NOT IN (?)", sub)
	}

	if id, ok := ctx.GetQuery("self_parent"); ok {
		sql := fmt.Sprintf(`
		WITH RECURSIVE cte AS(
			SELECT id,parent_id
			FROM node
			WHERE id=%s AND deleted_at = 0
			UNION ALL
			SELECT t.id,t.parent_id
			FROM cte
				INNER JOIN node t on cte.parent_id = t.id
			WHERE deleted_at = 0
		)
		SELECT
			id
		FROM cte
		`, id)
		sub := mysql.DB.Raw(sql)
		db = db.Where("id IN (?)", sub)
	}

	db = db.Order("name DESC")

	doGet[*model.Node](ctx, false, db, "", nodePostHooks...)
}
