package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/connector"
)

// Connect handles WebSocket connections for terminal sessions
// @Tags		connect
// @Success	200	{object}	HttpResponse
// @Param		w			query	int		false	"width"
// @Param		h			query	int		false	"height"
// @Param		dpi			query	int		false	"dpi"
// @Param		session_id	query	string	false	"session_id"
// @Success	200	{object}	HttpResponse{}
// @Router		/connect/:asset_id/:account_id/:protocol [get]
func (c *Controller) Connect(ctx *gin.Context) {
	connector.Connect(ctx)
}

// ConnectMonitor handles WebSocket connections for monitoring sessions
// @Tags		connect
// @Success	200	{object}	HttpResponse
// @Router		/connect/monitor/:session_id [get]
func (c *Controller) ConnectMonitor(ctx *gin.Context) {
	connector.ConnectMonitor(ctx)
}

// ConnectClose handles closing a session
// @Tags		connect
// @Success	200	{object}	HttpResponse
// @Router		/connect/close/:session_id [post]
func (c *Controller) ConnectClose(ctx *gin.Context) {
	connector.ConnectClose(ctx)
}
