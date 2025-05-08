package controller

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/errors"
)

var (
	sessionService = service.NewSessionService()

	sessionPostHooks = []postHook[*model.Session]{
		func(ctx *gin.Context, data []*model.Session) {
			if err := sessionService.AttachCmdCounts(ctx, data); err != nil {
			}
		},
		func(ctx *gin.Context, data []*model.Session) {
			sessionService.CalculateDurations(data)
		},
	}
)

// CreateSessionCommand godoc
//
//	@Tags		session
//	@Param		sessioncmd	body		model.SessionCmd	true	"SessionCmd"
//	@Success	200			{object}	HttpResponse
//	@Router		/session/cmd [post]
func (c *Controller) CreateSessionCmd(ctx *gin.Context) {
	data := &model.SessionCmd{}
	if err := ctx.BindJSON(data); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	if err := sessionService.CreateSessionCmd(ctx, data); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// GetSessions godoc
//
//	@Tags		session
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		search		query		string	false	"search"
//	@Param		status		query		int		false	"status, online=1, offline=2"
//	@Param		start		query		string	false	"start, RFC3339"
//	@Param		end			query		string	false	"end, RFC3339"
//	@Param		uid			query		int		false	"uid"
//	@Param		asset_id	query		int		false	"asset id"
//	@Param		client_ip	query		string	false	"client_ip"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Session}}
//	@Router		/session [get]
func (c *Controller) GetSessions(ctx *gin.Context) {
	db, err := sessionService.BuildQuery(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	doGet(ctx, false, db, "", sessionPostHooks...)
}

// GetSessionCmds godoc
//
//	@Tags		session
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		session_id	path		string	true	"session id"
//	@Param		search		query		string	true	"search"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.SessionCmd}}
//	@Router		/session/:session_id/cmd [get]
func (c *Controller) GetSessionCmds(ctx *gin.Context) {
	db := sessionService.BuildCmdQuery(ctx, ctx.Param("session_id"))
	doGet[*model.SessionCmd](ctx, false, db, "")
}

// GetSessionOptionAsset godoc
//
//	@Tags		session
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.SessionOptionAsset}}
//	@Router		/session/option/asset [get]
func (c *Controller) GetSessionOptionAsset(ctx *gin.Context) {
	opts, err := sessionService.GetSessionOptionAssets(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(opts))
}

// GetSessionOptionClientIp godoc
//
//	@Tags		session
//	@Success	200	{object}	HttpResponse{data=[]string}
//	@Router		/session/option/clientip [get]
func (c *Controller) GetSessionOptionClientIp(ctx *gin.Context) {
	opts, err := sessionService.GetSessionOptionClientIps(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(opts))
}

// CreateSessionReplay godoc
//
//	@Tags		session
//	@Param		session_id	path		string	true	"session id"
//	@Success	200			{object}	HttpResponse
//	@Router		/session/replay/:session_id [post]
func (c *Controller) CreateSessionReplay(ctx *gin.Context) {
	file, _, err := ctx.Request.FormFile("replay.cast")
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	sessionId := ctx.Param("session_id")
	if err := sessionService.CreateSessionReplay(ctx, sessionId, file); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// GetSessionReplay godoc
//
//	@Tags		session
//	@Param		session_id	path		string	true	"session id"
//	@Success	200			{object}	string
//	@Router		/session/replay/:session_id [get]
func (c *Controller) GetSessionReplay(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	filename, err := sessionService.GetSessionReplayFilename(ctx, sessionId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	ctx.FileAttachment(filepath.Join("/tmp/replay", filename), filename)
}
