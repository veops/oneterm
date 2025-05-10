package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/service"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

var prefService = service.DefaultUserPreferenceService

// GetPreference godoc
// @Summary Get user preferences
// @Description Get terminal preferences for the current user
// @Tags Preference
// @Success 200 {object} HttpResponse{data=model.UserPreference}
// @Router /preference [get]
func (c *Controller) GetPreference(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get preferences for current user
	pref, err := prefService.GetUserPreference(ctx, currentUser.Uid)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{
			Code: myErrors.ErrInternal,
			Data: map[string]any{"err": err},
		})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: pref,
	})
}

// UpdatePreference godoc
// @Summary Update user preferences
// @Description Update terminal preferences for the current user
// @Tags Preference
// @Param preference body model.UserPreference true "User preferences"
// @Success 200 {object} HttpResponse{data=model.UserPreference}
// @Router /preference [put]
func (c *Controller) UpdatePreference(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Create a temporary map to bind JSON data
	var prefMap map[string]any
	if err := ctx.ShouldBindBodyWithJSON(&prefMap); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{
			Code: myErrors.ErrInvalidArgument,
			Data: map[string]any{"err": err},
		})
		return
	}

	// Update preferences with the map data
	if err := prefService.UpdateUserPreference(ctx, currentUser.Uid, prefMap); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{
			Code: myErrors.ErrInternal,
			Data: map[string]any{"err": err},
		})
		return
	}

	// Get updated preferences
	updatedPref, err := prefService.GetUserPreference(ctx, currentUser.Uid)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{
			Code: myErrors.ErrInternal,
			Data: map[string]any{"err": err},
		})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: updatedPref,
	})
}
