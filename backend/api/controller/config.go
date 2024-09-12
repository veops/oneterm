package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	redis "github.com/veops/oneterm/cache"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/model"
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
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": acl.WRITE}})
		return
	}

	cfg := &model.Config{}
	if err := ctx.ShouldBindBodyWithJSON(cfg); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	cfg.Id = 0
	cfg.CreatorId = currentUser.GetUid()
	cfg.UpdaterId = currentUser.GetUid()

	if err := mysql.DB.Model(cfg).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("deleted_at = 0").Delete(&model.Config{}).Error; err != nil {
			return err
		}
		return tx.Create(cfg).Error
	}); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	model.GlobalConfig.Store(cfg)
	redis.SetEx(ctx, "config", cfg, time.Hour)

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
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": acl.READ}})
		return
	}

	cfg := &model.Config{}
	if err := mysql.DB.Model(cfg).First(&cfg).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(cfg))
}
