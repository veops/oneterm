package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
)

var (
	historyService = service.NewHistoryService()
)

// GetHistories godoc
//
//	@Tags		history
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		type		query		string	false	"type"	Enums(account, asset, command, gateway, node, public_key)
//	@Param		target_id	query		int		false	"target_id"
//	@Param		uid			query		int		false	"uid"
//	@Param		action_type	query		int		false	"create=1 delete=2 update=3"
//	@Param		start		query		string	false	"start time, RFC3339"
//	@Param		end			query		string	false	"end time, RFC3339"
//	@Param		search		query		string	false	"search"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.History}}
//	@Router		/history [get]
func (c *Controller) GetHistories(ctx *gin.Context) {
	db, err := historyService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	doGet[*model.History](ctx, false, db, "")
}

// GetSessions godoc
//
//	@Tags		session
//	@Success	200	{object}	HttpResponse{data=map[string]string}
//	@Router		/history/type/mapping [get]
func (c *Controller) GetHistoryTypeMapping(ctx *gin.Context) {
	mapping, err := historyService.GetTypeMapping(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(mapping))
}
