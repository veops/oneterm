package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

var (
	sessionPostHooks = []postHook[*model.Session]{
		func(ctx *gin.Context, data []*model.Session) {
			sessionIds := lo.Map(data, func(d *model.Session, _ int) string { return d.SessionId })
			if len(sessionIds) <= 0 {
				return
			}
			post := make([]*model.CmdCount, 0)
			if err := mysql.DB.
				Model(&model.SessionCmd{}).
				Select("session_id, COUNT(*) AS count").
				Where("session_id IN ?", sessionIds).
				Group("session_id").
				Find(&post).
				Error; err != nil {
				logger.L().Error("gateway posthookfailed", zap.Error(err))
				return
			}
			m := lo.SliceToMap(post, func(p *model.CmdCount) (string, int64) { return p.SessionId, p.Count })
			for _, d := range data {
				d.CmdCount = m[d.SessionId]
			}
		},
		func(ctx *gin.Context, data []*model.Session) {
			now := time.Now()
			for _, d := range data {
				t := now
				if d.ClosedAt != nil {
					t = *d.ClosedAt
				}
				d.Duration = int64(t.Sub(d.CreatedAt).Seconds())
			}
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
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	if err := mysql.DB.
		Create(data).
		Error; err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
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
	db := mysql.DB.Model(model.DefaultSession)
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		db = db.Where("uid = ?", currentUser.Uid)
	}
	db = filterSearch(ctx, db, "user_name", "asset_info", "gateway_info", "account_info")
	db, err := filterStartEnd(ctx, db)
	if err != nil {
		return
	}
	db = filterEqual(ctx, db, "status", "uid", "asset_id", "client_ip")

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
	db := mysql.DB.Model(&model.SessionCmd{})
	db = db.Where("session_id = ?", ctx.Param("session_id"))
	db = filterSearch(ctx, db, "cmd", "result")

	doGet[*model.SessionCmd](ctx, false, db, "")
}

// GetSessionOptionAsset godoc
//
//	@Tags		session
//	@Success	200	{object}	HttpResponse{data=ListData{list=[]model.SessionOptionAsset}}
//	@Router		/session/option/asset [get]
func (c *Controller) GetSessionOptionAsset(ctx *gin.Context) {
	opts := make([]*model.SessionOptionAsset, 0)
	if err := mysql.DB.
		Model(model.DefaultAsset).
		Select("id, name").
		Find(&opts).
		Error; err != nil {
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
	opts := make([]string, 0)
	if err := mysql.DB.
		Model(model.DefaultSession).
		Distinct("client_ip").
		Find(&opts).
		Error; err != nil {
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
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	f, err := os.Create(filepath.Join("/replay", fmt.Sprintf("%s.cast", ctx.Param("session_id"))))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	defer f.Close()
	f.Write(content)

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
	session := &model.Session{}
	if err := mysql.DB.Model(session).Where("session_id = ?", sessionId).First(session).Error; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
	}
	filename := sessionId
	if session.IsSsh() {
		filename += ".cast"
	}
	ctx.FileAttachment(filepath.Join("/replay", filename), filename)
}
