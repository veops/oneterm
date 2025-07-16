package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

// CreateAuthorizationV2 godoc
//
//	@Tags		authorization_v2
//	@Param		authorization	body		model.AuthorizationV2	true	"authorization rule"
//	@Success	200				{object}	HttpResponse
//	@Router		/authorization_v2 [post]
func (c *Controller) CreateAuthorizationV2(ctx *gin.Context) {
	auth := &model.AuthorizationV2{}
	err := ctx.ShouldBindBodyWithJSON(auth)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	authV2Service := service.NewAuthorizationV2Service()

	// Check permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
	}

	// Create the rule
	err = authV2Service.CreateRule(ctx, auth)
	if err != nil {
		if ctx.IsAborted() {
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": auth.GetId(),
		},
	})
}

// UpdateAuthorizationV2 godoc
//
//	@Tags		authorization_v2
//	@Param		id				path		int						true	"authorization id"
//	@Param		authorization	body		model.AuthorizationV2	true	"authorization rule"
//	@Success	200				{object}	HttpResponse
//	@Router		/authorization_v2/:id [put]
func (c *Controller) UpdateAuthorizationV2(ctx *gin.Context) {
	authId := cast.ToInt(ctx.Param("id"))
	auth := &model.AuthorizationV2{}
	err := ctx.ShouldBindBodyWithJSON(auth)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	auth.Id = authId
	authV2Service := service.NewAuthorizationV2Service()

	// Check permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
	}

	// Update the rule
	err = authV2Service.UpdateRule(ctx, auth)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		}
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": auth.GetId(),
		},
	})
}

// DeleteAuthorizationV2 godoc
//
//	@Tags		authorization_v2
//	@Param		id	path		int	true	"authorization id"
//	@Success	200	{object}	HttpResponse
//	@Router		/authorization_v2/:id [delete]
func (c *Controller) DeleteAuthorizationV2(ctx *gin.Context) {
	authId := cast.ToInt(ctx.Param("id"))

	authV2Service := service.NewAuthorizationV2Service()

	// Check permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
	}

	// Delete the rule
	if err := authV2Service.DeleteRule(ctx, authId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		}
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: map[string]any{
			"id": authId,
		},
	})
}

// GetAuthorizationV2 godoc
//
//	@Tags		authorization_v2
//	@Param		id	path		int	true	"authorization id"
//	@Success	200	{object}	HttpResponse{data=model.AuthorizationV2}
//	@Router		/authorization_v2/:id [get]
func (c *Controller) GetAuthorizationV2(ctx *gin.Context) {
	authId := cast.ToInt(ctx.Param("id"))

	authV2Service := service.NewAuthorizationV2Service()

	// Get the rule
	auth, err := authV2Service.GetRuleById(ctx, authId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		}
		return
	}

	if auth == nil {
		ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "rule not found"}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: auth,
	})
}

// CloneAuthorizationV2 godoc
//
//	@Tags		authorization_v2
//	@Param		id		path		int		true	"source authorization id"
//	@Param		body	body		object	true	"clone request"
//	@Success	200		{object}	HttpResponse{data=model.AuthorizationV2}
//	@Router		/authorization_v2/:id/clone [post]
func (c *Controller) CloneAuthorizationV2(ctx *gin.Context) {
	sourceId := cast.ToInt(ctx.Param("id"))

	// Parse request body
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	err := ctx.ShouldBindBodyWithJSON(&req)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	authV2Service := service.NewAuthorizationV2Service()

	// Check permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.GRANT}})
		return
	}

	// Clone the rule
	clonedRule, err := authV2Service.CloneRule(ctx, sourceId, req.Name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "source rule not found"}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		}
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: clonedRule,
	})
}

// GetAuthorizationsV2 godoc
//
//	@Tags		authorization_v2
//	@Param		page_index	query		int		false	"page index"
//	@Param		page_size	query		int		false	"page size"
//	@Param		enabled		query		bool	false	"filter by enabled status"
//	@Param		search		query		string	false	"search by name or description"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.AuthorizationV2}}
//	@Router		/authorization_v2 [get]
func (c *Controller) GetAuthorizationsV2(ctx *gin.Context) {
	pageIndex := cast.ToInt(ctx.DefaultQuery("page_index", "1"))
	pageSize := cast.ToInt(ctx.DefaultQuery("page_size", "20"))
	enabled := ctx.Query("enabled")
	search := ctx.Query("search")

	// Check permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	authV2Service := service.NewAuthorizationV2Service()

	// Build query with filters
	db, err := authV2Service.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply filters
	if enabled != "" {
		db = db.Where("enabled = ?", enabled == "true")
	}
	if search != "" {
		db = db.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Get total count
	var count int64
	if err := db.Count(&count).Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Apply pagination
	offset := (pageIndex - 1) * pageSize
	var auths []*model.AuthorizationV2
	if err := db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&auths).Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Convert to slice of any type
	authsAny := make([]any, 0, len(auths))
	for _, a := range auths {
		authsAny = append(authsAny, a)
	}

	result := &ListData{
		Count: count,
		List:  authsAny,
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: result,
	})
}

// CheckPermissionV2 godoc
//
//	@Tags		authorization_v2
//	@Param		request	body		CheckPermissionRequest	true	"permission check request"
//	@Success	200		{object}	HttpResponse{data=model.AuthResult}
//	@Router		/authorization_v2/check [post]
func (c *Controller) CheckPermissionV2(ctx *gin.Context) {
	var req CheckPermissionRequest
	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument})
		return
	}

	result, err := service.DefaultAuthService.CheckPermission(ctx, req.NodeId, req.AssetId, req.AccountId, req.Action)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{Data: result})
}

// CheckPermissionRequest represents a permission check request
type CheckPermissionRequest struct {
	NodeId    int              `json:"node_id" binding:"gte=0"`
	AssetId   int              `json:"asset_id" binding:"gte=0"`
	AccountId int              `json:"account_id" binding:"gte=0"`
	Action    model.AuthAction `json:"action" binding:"required"`
}

// AuthorizationV2 hooks
var (
	authV2PreHooks = []preHook[*model.AuthorizationV2]{
		func(ctx *gin.Context, data *model.AuthorizationV2) {
			authV2Service := service.NewAuthorizationV2Service()
			if err := authV2Service.ValidateRule(ctx, data); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err.Error()}})
			}
		},
	}

	authV2PostHooks = []postHook[*model.AuthorizationV2]{
		func(ctx *gin.Context, data []*model.AuthorizationV2) {
			// Add any post-processing logic here
		},
	}
)
