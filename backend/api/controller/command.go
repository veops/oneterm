package controller

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
)

var (
	commandPreHooks = []preHook[*model.Command]{
		func(ctx *gin.Context, data *model.Command) {
			if !data.IsRe {
				return
			}
			_, err := regexp.Compile(data.Cmd)
			if err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": err}})
			}
		},
	}
	commandDcs = []deleteCheck{
		func(ctx *gin.Context, id int) {
			assetName := ""
			err := mysql.DB.
				Model(model.DefaultAsset).
				Select("name").
				Where(fmt.Sprintf("JSON_CONTAINS(cmd_ids, '%d')", id)).
				First(&assetName).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
			code := lo.Ternary(err == nil, http.StatusBadRequest, http.StatusInternalServerError)
			err = lo.Ternary[error](err == nil, &ApiError{Code: ErrHasDepency, Data: map[string]any{"name": assetName}}, err)
			ctx.AbortWithError(code, err)
		},
	}
)

// CreateCommand godoc
//
//	@Tags		command
//	@Param		command	body		model.Command	true	"command"
//	@Success	200		{object}	HttpResponse
//	@Router		/command [post]
func (c *Controller) CreateCommand(ctx *gin.Context) {
	doCreate(ctx, true, &model.Command{}, conf.RESOURCE_COMMAND, commandPreHooks...)
}

// DeleteCommand godoc
//
//	@Tags		command
//	@Param		id	path		int	true	"command id"
//	@Success	200	{object}	HttpResponse
//	@Router		/command/:id [delete]
func (c *Controller) DeleteCommand(ctx *gin.Context) {
	doDelete(ctx, true, &model.Command{}, conf.RESOURCE_COMMAND, commandDcs...)
}

// UpdateCommand godoc
//
//	@Tags		command
//	@Param		id		path		int				true	"command id"
//	@Param		command	body		model.Command	true	"command"
//	@Success	200		{object}	HttpResponse
//	@Router		/command/:id [put]
func (c *Controller) UpdateCommand(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Command{}, conf.RESOURCE_COMMAND, commandPreHooks...)
}

// GetCommands godoc
//
//	@Tags		command
//	@Param		page_index	query		int		true	"command id"
//	@Param		page_size	query		int		true	"command id"
//	@Param		search		query		string	false	"name or cmds"
//	@Param		id			query		int		false	"command id"
//	@Param		ids			query		string	false	"command ids"
//	@Param		name		query		string	false	"command name"
//	@Param		enable		query		int		false	"command enable"
//	@Param		info		query		bool	false	"is info mode"
//	@Param		search		query		string	false	"name or cmds"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Command}}
//	@Router		/command [get]
func (c *Controller) GetCommands(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	info := cast.ToBool(ctx.Query("info"))

	db := mysql.DB.Model(&model.Command{})
	db = filterEqual(ctx, db, "id", "enable")
	db = filterLike(ctx, db, "name")
	db = filterSearch(ctx, db, "name", "cmds")
	if q, ok := ctx.GetQuery("ids"); ok {
		db = db.Where("id IN ?", lo.Map(strings.Split(q, ","), func(s string, _ int) int { return cast.ToInt(s) }))
	}

	if info && !acl.IsAdmin(currentUser) {
		//rs := make([]*acl.Resource, 0)
		rs, err := acl.GetRoleResources(ctx, currentUser.Acl.Rid, conf.RESOURCE_AUTHORIZATION)
		if err != nil {
			handleRemoteErr(ctx, err)
			return
		}
		sub := mysql.DB.
			Model(&model.Authorization{}).
			Select("DISTINCT asset_id").
			Where("resource_id IN ?", lo.Map(rs, func(r *acl.Resource, _ int) int { return r.ResourceId }))
		cmdIds := make([]model.Slice[int], 0)
		if err = mysql.DB.
			Model(model.DefaultAsset).
			Select("cmd_ids").
			Where("id IN (?)", sub).
			Find(&cmdIds).
			Error; err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		}

		ids := make([]int, 0)
		for _, s := range cmdIds {
			ids = append(ids, s...)
		}

		db = db.Where("id IN ?", lo.Uniq(ids))
	}

	db = db.Order("name")

	doGet[*model.Command](ctx, !info, db, conf.RESOURCE_COMMAND)
}
