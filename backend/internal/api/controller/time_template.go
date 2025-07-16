package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	pkgErrors "github.com/veops/oneterm/pkg/errors"
)

var (
	timeTemplateService = service.NewTimeTemplateService()
)

// CreateTimeTemplate godoc
//
//	@Tags		time_template
//	@Param		template	body		model.TimeTemplate	true	"time template"
//	@Success	200		{object}	HttpResponse
//	@Router		/time_template [post]
func (c *Controller) CreateTimeTemplate(ctx *gin.Context) {
	// Time templates require admin permission - no ACL needed
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &pkgErrors.ApiError{Code: pkgErrors.ErrNoPerm, Data: map[string]any{"perm": "admin"}})
		return
	}

	template := &model.TimeTemplate{}
	doCreate(ctx, false, template, "", timeTemplatePreHooks...)
}

// DeleteTimeTemplate godoc
//
//	@Tags		time_template
//	@Param		id	path		int	true	"template id"
//	@Success	200	{object}	HttpResponse
//	@Router		/time_template/:id [delete]
func (c *Controller) DeleteTimeTemplate(ctx *gin.Context) {
	// Time templates require admin permission - no ACL needed
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &pkgErrors.ApiError{Code: pkgErrors.ErrNoPerm, Data: map[string]any{"perm": "admin"}})
		return
	}

	doDelete(ctx, false, &model.TimeTemplate{}, "", timeTemplateDeleteChecks...)
}

// UpdateTimeTemplate godoc
//
//	@Tags		time_template
//	@Param		id		path		int					true	"template id"
//	@Param		template	body		model.TimeTemplate	true	"time template"
//	@Success	200		{object}	HttpResponse
//	@Router		/time_template/:id [put]
func (c *Controller) UpdateTimeTemplate(ctx *gin.Context) {
	// Time templates require admin permission - no ACL needed
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &pkgErrors.ApiError{Code: pkgErrors.ErrNoPerm, Data: map[string]any{"perm": "admin"}})
		return
	}

	template := &model.TimeTemplate{}
	doUpdate(ctx, false, template, "", timeTemplatePreHooks...)
}

// GetTimeTemplates godoc
//
//	@Tags		time_template
//	@Param		page_index	query		int		false	"page index"
//	@Param		page_size	query		int		false	"page size"
//	@Param		search		query		string	false	"search by name or description"
//	@Param		category	query		string	false	"template category"
//	@Param		active		query		bool	false	"filter by active status"
//	@Param		info		query		bool	false	"info mode"
//	@Success	200		{object}	HttpResponse{data=[]model.TimeTemplate}
//	@Router		/time_template [get]
func (c *Controller) GetTimeTemplates(ctx *gin.Context) {
	info := cast.ToBool(ctx.Query("info"))

	// Build base query using service layer with all filters
	db, err := timeTemplateService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	doGet(ctx, !info, db, "", timeTemplatePostHooks...)
}

// GetBuiltInTimeTemplates godoc
//
//	@Tags		time_template
//	@Success	200	{object}	HttpResponse{data=[]model.TimeTemplate}
//	@Router		/time_template/builtin [get]
func (c *Controller) GetBuiltInTimeTemplates(ctx *gin.Context) {
	templates, err := timeTemplateService.GetBuiltInTemplates(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(templates))
}

// CheckTimeAccess godoc
//
//	@Tags		time_template
//	@Param		request	body		CheckTimeAccessRequest	true	"time access check request"
//	@Success	200		{object}	HttpResponse{data=CheckTimeAccessResponse}
//	@Router		/time_template/check [post]
func (c *Controller) CheckTimeAccess(ctx *gin.Context) {
	var req CheckTimeAccessRequest
	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument})
		return
	}

	allowed, err := timeTemplateService.CheckTimeAccess(ctx, req.TemplateID, req.Timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	response := CheckTimeAccessResponse{
		Allowed:    allowed,
		TemplateID: req.TemplateID,
		Timezone:   req.Timezone,
		CheckedAt:  "now",
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(response))
}

// InitBuiltInTemplates godoc
//
//	@Tags		time_template
//	@Success	200	{object}	HttpResponse
//	@Router		/time_template/init [post]
func (c *Controller) InitBuiltInTemplates(ctx *gin.Context) {
	if err := timeTemplateService.InitializeBuiltInTemplates(ctx); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData("Built-in time templates initialized successfully"))
}

// CheckTimeAccessRequest represents a time access check request
type CheckTimeAccessRequest struct {
	TemplateID int    `json:"template_id" binding:"required,gt=0"`
	Timezone   string `json:"timezone"`
}

// CheckTimeAccessResponse represents a time access check response
type CheckTimeAccessResponse struct {
	Allowed    bool   `json:"allowed"`
	TemplateID int    `json:"template_id"`
	Timezone   string `json:"timezone"`
	CheckedAt  string `json:"checked_at"`
}

// Time template hooks
var (
	timeTemplatePreHooks = []preHook[*model.TimeTemplate]{
		func(ctx *gin.Context, data *model.TimeTemplate) {
			if err := timeTemplateService.ValidateTimeTemplate(data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument, Data: map[string]any{"err": err.Error()}})
			}
		},
	}

	timeTemplatePostHooks = []postHook[*model.TimeTemplate]{
		func(ctx *gin.Context, data []*model.TimeTemplate) {
			// Add any post-processing logic here
		},
	}

	timeTemplateDeleteChecks = []deleteCheck{
		func(ctx *gin.Context, id int) {
			// Check if template is built-in
			template, err := timeTemplateService.GetTimeTemplate(ctx, id)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, &pkgErrors.ApiError{Code: pkgErrors.ErrInternal, Data: map[string]any{"err": err}})
				return
			}
			if template != nil && template.IsBuiltIn {
				ctx.AbortWithError(http.StatusBadRequest, &pkgErrors.ApiError{Code: pkgErrors.ErrInvalidArgument, Data: map[string]any{"err": "cannot delete built-in time template"}})
			}
		},
	}
)
