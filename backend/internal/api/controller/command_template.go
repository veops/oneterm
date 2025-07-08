package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/config"
	pkgErrors "github.com/veops/oneterm/pkg/errors"
)

var (
	commandTemplateService = service.NewCommandTemplateService()
)

// CreateCommandTemplate godoc
//
//	@Tags		command_template
//	@Param		template	body		model.CommandTemplate	true	"command template"
//	@Success	200		{object}	HttpResponse
//	@Router		/command_template [post]
func (c *Controller) CreateCommandTemplate(ctx *gin.Context) {
	template := &model.CommandTemplate{}
	doCreate(ctx, true, template, config.RESOURCE_AUTHORIZATION, commandTemplatePreHooks...)
}

// DeleteCommandTemplate godoc
//
//	@Tags		command_template
//	@Param		id	path		int	true	"template id"
//	@Success	200	{object}	HttpResponse
//	@Router		/command_template/:id [delete]
func (c *Controller) DeleteCommandTemplate(ctx *gin.Context) {
	doDelete(ctx, true, &model.CommandTemplate{}, config.RESOURCE_AUTHORIZATION, commandTemplateDeleteChecks...)
}

// UpdateCommandTemplate godoc
//
//	@Tags		command_template
//	@Param		id		path		int					true	"template id"
//	@Param		template	body		model.CommandTemplate	true	"command template"
//	@Success	200		{object}	HttpResponse
//	@Router		/command_template/:id [put]
func (c *Controller) UpdateCommandTemplate(ctx *gin.Context) {
	template := &model.CommandTemplate{}
	doUpdate(ctx, true, template, config.RESOURCE_AUTHORIZATION, commandTemplatePreHooks...)
}

// GetCommandTemplates godoc
//
//	@Tags		command_template
//	@Param		page_index	query		int		false	"page index"
//	@Param		page_size	query		int		false	"page size"
//	@Param		category	query		string	false	"template category"
//	@Param		builtin		query		bool	false	"filter by builtin status"
//	@Param		info		query		bool	false	"info mode"
//	@Success	200		{object}	HttpResponse{data=[]model.CommandTemplate}
//	@Router		/command_template [get]
func (c *Controller) GetCommandTemplates(ctx *gin.Context) {
	info := cast.ToBool(ctx.Query("info"))

	// Build base query using service layer
	db, err := commandTemplateService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply filters
	if category := ctx.Query("category"); category != "" {
		db = db.Where("category = ?", category)
	}

	if builtinStr := ctx.Query("builtin"); builtinStr != "" {
		builtin := cast.ToBool(builtinStr)
		db = db.Where("is_builtin = ?", builtin)
	}

	doGet(ctx, !info, db, config.RESOURCE_AUTHORIZATION, commandTemplatePostHooks...)
}

// GetBuiltInCommandTemplates godoc
//
//	@Tags		command_template
//	@Success	200	{object}	HttpResponse{data=[]model.CommandTemplate}
//	@Router		/command_template/builtin [get]
func (c *Controller) GetBuiltInCommandTemplates(ctx *gin.Context) {
	templates, err := commandTemplateService.GetBuiltInTemplates(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(templates))
}

// GetTemplateCommands godoc
//
//	@Tags		command_template
//	@Param		id	path		int	true	"template id"
//	@Success	200	{object}	HttpResponse{data=[]model.Command}
//	@Router		/command_template/:id/commands [get]
func (c *Controller) GetTemplateCommands(ctx *gin.Context) {
	id := cast.ToInt(ctx.Param("id"))
	if id == 0 {
		ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument, Data: map[string]any{"err": "invalid template id"}})
		return
	}

	commands, err := commandTemplateService.GetTemplateCommands(ctx, id)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(commands))
}

// Command template hooks
var (
	commandTemplatePreHooks = []preHook[*model.CommandTemplate]{
		func(ctx *gin.Context, data *model.CommandTemplate) {
			if err := commandTemplateService.ValidateCommandTemplate(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument, Data: map[string]any{"err": err.Error()}})
			}
		},
	}

	commandTemplatePostHooks = []postHook[*model.CommandTemplate]{
		func(ctx *gin.Context, data []*model.CommandTemplate) {
			// Add any post-processing logic here
		},
	}

	commandTemplateDeleteChecks = []deleteCheck{
		func(ctx *gin.Context, id int) {
			// Check if template is built-in
			template, err := commandTemplateService.GetCommandTemplate(ctx, id)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
				return
			}
			if template != nil && template.IsBuiltin {
				ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument, Data: map[string]any{"err": "cannot delete built-in command template"}})
			}
		},
	}
)
