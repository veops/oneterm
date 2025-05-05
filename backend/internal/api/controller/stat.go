package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/service"
)

var (
	statService = service.NewStatService()
)

// StatAssetType godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAssetType}}
//	@Router		/stat/assettype [get]
func (c *Controller) StatAssetType(ctx *gin.Context) {
	stat, err := statService.GetAssetTypes(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatCount godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=model.StatCount}
//	@Router		/stat/count [get]
func (c *Controller) StatCount(ctx *gin.Context) {
	stat, err := statService.GetStatCount(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(stat))
}

// StatAccount godoc
//
//	@Tags		stat
//	@Param		type		query		string	true	"account name" Enums(day, week, month)
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAccount}}
//	@Router		/stat/account [get]
func (c *Controller) StatAccount(ctx *gin.Context) {
	timeRange := ctx.Query("type")

	stat, err := statService.GetStatAccount(ctx, timeRange)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatAsset godoc
//
//	@Tags		stat
//	@Param		type		query		string	true	"account name" Enums(day, week, month)
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatAsset}}
//	@Router		/stat/asset [get]
func (c *Controller) StatAsset(ctx *gin.Context) {
	timeRange := ctx.Query("type")

	stat, err := statService.GetStatAsset(ctx, timeRange)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// StatCountOfUser godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=model.StatCountOfUser}
//	@Router		/stat/count/ofuser [get]
func (c *Controller) StatCountOfUser(ctx *gin.Context) {
	stat, err := statService.GetStatCountOfUser(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(stat))
}

// StatRankOfUser godoc
//
//	@Tags		stat
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.StatRankOfUser}}
//	@Router		/stat/rank/ofuser [get]
func (c *Controller) StatRankOfUser(ctx *gin.Context) {
	stat, err := statService.GetStatRankOfUser(ctx, 10) // Limit to top 10
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(toListData(stat)))
}

// Helper functions

// toListData converts a slice to a ListData struct
func toListData[T any](data []T) *ListData {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = v
	}
	return &ListData{
		Count: int64(len(data)),
		List:  items,
	}
}
