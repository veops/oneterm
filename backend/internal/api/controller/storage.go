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

var storageService = service.DefaultStorageService

// ListStorageConfigs godoc
//
//	@Tags		storage
//	@Summary	List all storage configurations
//	@Param		page_index	query		int		false	"page_index"
//	@Param		page_size	query		int		false	"page_size"
//	@Param		search		query		string	false	"search"
//	@Param		type		query		string	false	"storage type filter"
//	@Param		enabled		query		string	false	"enabled filter (true/false)"
//	@Param		primary		query		string	false	"primary filter (true/false)"
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StorageConfig}}
//	@Router		/storage/configs [get]
func (c *Controller) ListStorageConfigs(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	db := storageService.BuildQuery(ctx)
	doGet[*model.StorageConfig](ctx, false, db, "")
}

// GetStorageConfig godoc
//
//	@Tags		storage
//	@Summary	Get storage configuration by ID
//	@Param		id	path		int	true	"Storage ID"
//	@Success	200		{object}	HttpResponse{data=model.StorageConfig}
//	@Router		/storage/configs/{id} [get]
func (c *Controller) GetStorageConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	baseService := service.NewBaseService()
	id, err := cast.ToIntE(ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	config := &model.StorageConfig{}
	if err := baseService.GetById(ctx, id, config); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusNotFound, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": "storage config not found"}})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{Data: config})
}

// CreateStorageConfig godoc
//
//	@Tags		storage
//	@Summary	Create a new storage configuration
//	@Param		config	body		model.StorageConfig	true	"Storage configuration"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/configs [post]
func (c *Controller) CreateStorageConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	doCreate(ctx, false, &model.StorageConfig{}, "", func(ctx *gin.Context, config *model.StorageConfig) {
		// Custom validation for storage config
		if err := validateStorageConfig(config); err != nil {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
			return
		}

		// Initialize storage provider after creation
		if provider, err := storageService.CreateProvider(config); err == nil {
			// Test connection
			if err := provider.HealthCheck(ctx); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
				return
			}
		}
	})
}

// UpdateStorageConfig godoc
//
//	@Tags		storage
//	@Summary	Update an existing storage configuration
//	@Param		id	path		int				true	"Storage ID"
//	@Param		config	body		model.StorageConfig	true	"Storage configuration"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/configs/{id} [put]
func (c *Controller) UpdateStorageConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	doUpdate(ctx, false, &model.StorageConfig{}, "", func(ctx *gin.Context, config *model.StorageConfig) {
		// Custom validation for storage config
		if err := validateStorageConfig(config); err != nil {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
			return
		}

		// Test connection after update
		if provider, err := storageService.CreateProvider(config); err == nil {
			if err := provider.HealthCheck(ctx); err != nil {
				ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
				return
			}
		}
	})
}

// DeleteStorageConfig godoc
//
//	@Tags		storage
//	@Summary	Delete a storage configuration
//	@Param		id	path		int	true	"Storage ID"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/configs/{id} [delete]
func (c *Controller) DeleteStorageConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	doDelete(ctx, false, &model.StorageConfig{}, "", func(ctx *gin.Context, id int) {
		// Custom validation: check if it's the primary storage
		config := &model.StorageConfig{}
		baseService := service.NewBaseService()
		if err := baseService.GetById(ctx, id, config); err == nil && config.IsPrimary {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "cannot delete primary storage"}})
			return
		}
	})
}

// TestStorageConnection godoc
//
//	@Tags		storage
//	@Summary	Test storage connection
//	@Param		config	body		model.StorageConfig	true	"Storage configuration to test"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/test-connection [post]
func (c *Controller) TestStorageConnection(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	config := &model.StorageConfig{}
	if err := ctx.ShouldBindJSON(config); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Create a temporary provider to test connection
	provider, err := storageService.CreateProvider(config)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Perform health check
	if err := provider.HealthCheck(ctx); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// GetStorageHealth godoc
//
//	@Tags		storage
//	@Summary	Get health status of all storage providers
//	@Success	200	{object}	HttpResponse{data=map[string]any}
//	@Router		/storage/health [get]
func (c *Controller) GetStorageHealth(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	healthResults := storageService.HealthCheck(ctx)

	// Convert error map to a more API-friendly format
	healthStatus := make(map[string]map[string]interface{})
	for name, err := range healthResults {
		healthStatus[name] = map[string]interface{}{
			"healthy": err == nil,
			"error":   err,
		}
	}

	ctx.JSON(http.StatusOK, HttpResponse{Data: healthStatus})
}

// SetPrimaryStorage godoc
//
//	@Tags		storage
//	@Summary	Set a storage provider as primary
//	@Param		id	path		int	true	"Storage ID"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/configs/{id}/set-primary [put]
func (c *Controller) SetPrimaryStorage(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	id, err := cast.ToIntE(ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Get current config
	baseService := service.NewBaseService()
	config := &model.StorageConfig{}
	if err := baseService.GetById(ctx, id, config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Update to set as primary
	config.IsPrimary = true
	config.UpdaterId = currentUser.GetUid()

	if err := storageService.UpdateStorageConfig(ctx, config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// ToggleStorageProvider godoc
//
//	@Tags		storage
//	@Summary	Enable or disable a storage provider
//	@Param		id	path		int	true	"Storage ID"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/storage/configs/{id}/toggle [put]
func (c *Controller) ToggleStorageProvider(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	id, err := cast.ToIntE(ctx.Param("id"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Get current config
	baseService := service.NewBaseService()
	config := &model.StorageConfig{}
	if err := baseService.GetById(ctx, id, config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Toggle enabled status
	config.Enabled = !config.Enabled
	config.UpdaterId = currentUser.GetUid()

	if err := storageService.UpdateStorageConfig(ctx, config); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{Data: map[string]bool{"enabled": config.Enabled}})
}

// validateStorageConfig validates storage configuration
func validateStorageConfig(config *model.StorageConfig) error {
	if config.Name == "" {
		return &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "storage name is required"}}
	}
	if config.Type == "" {
		return &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "storage type is required"}}
	}
	return nil
}
