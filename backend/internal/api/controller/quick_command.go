package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

var (
	quickCommand = service.DefaultQuickCommand

	quickCommandPreHooks = []preHook[*model.QuickCommand]{
		// Check global command permission
		func(ctx *gin.Context, cmd *model.QuickCommand) {
			currentUser, _ := acl.GetSessionFromCtx(ctx)
			if cmd.IsGlobal && !acl.IsAdmin(currentUser) {
				ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm})
				return
			}
		},
		// Set creator ID
		func(ctx *gin.Context, cmd *model.QuickCommand) {
			currentUser, _ := acl.GetSessionFromCtx(ctx)
			cmd.CreatorId = currentUser.Uid
		},
	}
)

// CreateQuickCommand godoc
// @Description Create a new quick command
// @Tags QuickCommand
// @Param quick_command body model.QuickCommand true "Quick command data"
// @Success 200 {object} HttpResponse
// @Router /quick_command [post]
func (c *Controller) CreateQuickCommand(ctx *gin.Context) {
	doCreate(ctx, false, &model.QuickCommand{}, "", quickCommandPreHooks...)
}

// GetQuickCommands godoc
// @Description Get all quick commands available to the user
// @Tags QuickCommand
// @Param		page_index	query		int		true	"page index"
// @Param		page_size	query		int		true	"page size"
// @Param		search		query		string	false	"name or command"
// @Param		id			query		int		false	"command id"
// @Param		name		query		string	false	"command name"
// @Success 200 {object} HttpResponse{data=ListData{list=[]model.QuickCommand}}
// @Router /quick_command [get]
func (c *Controller) GetQuickCommands(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	db := quickCommand.BuildQuery(ctx)

	// Add filter for non-admin users
	if !acl.IsAdmin(currentUser) {
		db = db.Where("creator_id = ? OR is_global = ?", currentUser.Uid, true)
	}

	doGet[*model.QuickCommand](ctx, false, db, "")
}

// DeleteQuickCommand godoc
// @Description Delete a quick command by ID
// @Tags QuickCommand
// @Param id path int true "Quick command ID"
// @Success 200 {object} HttpResponse
// @Router /quick_command/{id} [delete]
func (c *Controller) DeleteQuickCommand(ctx *gin.Context) {
	doDelete(ctx, false, &model.QuickCommand{}, "")
}

// UpdateQuickCommand godoc
// @Description Update an existing quick command by ID
// @Tags QuickCommand
// @Param id path int true "Quick command ID"
// @Param quick_command body model.QuickCommand true "Updated quick command data"
// @Success 200 {object} HttpResponse
// @Router /quick_command/{id} [put]
func (c *Controller) UpdateQuickCommand(ctx *gin.Context) {
	id := cast.ToInt(ctx.Param("id"))
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get existing command
	cmd, err := quickCommand.GetById(ctx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Check permissions
	if cmd.CreatorId != currentUser.Uid && !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm})
		return
	}

	// Bind new data
	newCmd := &model.QuickCommand{}
	if err := ctx.ShouldBindBodyWithJSON(newCmd); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Check global command permission
	if newCmd.IsGlobal && !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm})
		return
	}

	// Update command
	newCmd.Id = id
	newCmd.CreatorId = cmd.CreatorId // Preserve original creator
	if err := quickCommand.Update(ctx, newCmd); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": id,
		},
	})
}
