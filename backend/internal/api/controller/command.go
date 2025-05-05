package controller

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

var (
	commandService = service.NewCommandService()

	commandPreHooks = []preHook[*model.Command]{
		func(ctx *gin.Context, data *model.Command) {
			if !data.IsRe {
				return
			}
			_, err := regexp.Compile(data.Cmd)
			if err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrBadRequest, Data: map[string]any{"err": err}})
			}
		},
	}
	commandDcs = []deleteCheck{
		func(ctx *gin.Context, id int) {
			assetName, err := commandService.CheckDependencies(ctx, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return
				}
				ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
				return
			}

			if assetName != "" {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrHasDepency, Data: map[string]any{"name": assetName}})
			}
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
	doCreate(ctx, true, &model.Command{}, config.RESOURCE_COMMAND, commandPreHooks...)
}

// DeleteCommand godoc
//
//	@Tags		command
//	@Param		id	path		int	true	"command id"
//	@Success	200	{object}	HttpResponse
//	@Router		/command/:id [delete]
func (c *Controller) DeleteCommand(ctx *gin.Context) {
	doDelete(ctx, true, &model.Command{}, config.RESOURCE_COMMAND, commandDcs...)
}

// UpdateCommand godoc
//
//	@Tags		command
//	@Param		id		path		int				true	"command id"
//	@Param		command	body		model.Command	true	"command"
//	@Success	200		{object}	HttpResponse
//	@Router		/command/:id [put]
func (c *Controller) UpdateCommand(ctx *gin.Context) {
	doUpdate(ctx, true, &model.Command{}, config.RESOURCE_COMMAND, commandPreHooks...)
}

// GetCommands godoc
//
//	@Tags		command
//	@Param		page_index	query		int		true	"command id"
//	@Param		page_size	query		int		true	"command id"
//	@Param		search		query		string	false	"name or cmd"
//	@Param		id			query		int		false	"command id"
//	@Param		ids			query		string	false	"command ids"
//	@Param		name		query		string	false	"command name"
//	@Param		enable		query		int		false	"command enable"
//	@Param		info		query		bool	false	"is info mode"
//	@Param		search		query		string	false	"name or cmd"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Command}}
//	@Router		/command [get]
func (c *Controller) GetCommands(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	info := cast.ToBool(ctx.Query("info"))

	db, err := commandService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	if info && !acl.IsAdmin(currentUser) {
		commandIds, err := commandService.GetAuthorizedCommandIds(ctx, currentUser)
		if err != nil {
			handleRemoteErr(ctx, err)
			return
		}

		if len(commandIds) > 0 {
			db = db.Where("id IN ?", commandIds)
		}
	}

	db = db.Order("name")

	doGet[*model.Command](ctx, !info, db, config.RESOURCE_COMMAND)
}
