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

var (
	configService = service.NewConfigService()
)

// PostConfig godoc
//
//	@Tags		config
//	@Param		command	body		model.Config	true	"config"
//	@Success	200		{object}	HttpResponse{}
//	@Router		/config [post]
func (c *Controller) PostConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	cfg := &model.Config{}
	if err := ctx.ShouldBindBodyWithJSON(cfg); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	cfg.CreatorId = currentUser.GetUid()
	cfg.UpdaterId = currentUser.GetUid()

	if err := configService.SaveConfig(ctx, cfg); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// GetConfig godoc
//
//	@Tags		config
//	@Param		info	query		bool	false	"is info mode"
//	@Success	200		{object}	HttpResponse{data=model.Config}
//	@Router		/config [get]
func (c *Controller) GetConfig(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !cast.ToBool(ctx.Query("info")) && !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	cfg, err := configService.GetConfig(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInternal, Data: map[string]any{"err": err}})
			return
		}
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(cfg))
}
