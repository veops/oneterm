package connector

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/connector/protocols"
	"github.com/veops/oneterm/internal/connector/protocols/db"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	byteClearAll = []byte("\x15\r")
)

func Connect(ctx *gin.Context) {
	ctx.Set("sessionType", model.SESSIONTYPE_WEB)

	ws, err := protocols.Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	if shareErr, exists := ctx.Get("shareErr"); exists && shareErr != nil {
		if apiErr, ok := shareErr.(*myErrors.ApiError); ok {
			errMsg := apiErr.MessageWithCtx(ctx)
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n \033[31m Invalid share link: %s \x1b[0m", errMsg)))
			return
		}
	}

	var sess *gsession.Session
	defer func() {
		protocols.HandleError(ctx, sess, err, ws, nil)
	}()

	sess, err = DoConnect(ctx, ws)
	if err != nil {
		return
	}

	if sess.IsGuacd() {
		protocols.HandleGuacd(sess)
	} else {
		HandleTerm(sess, ctx)
	}
}

func ConnectMonitor(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sessionId := ctx.Param("session_id")
	var sess *gsession.Session
	ws, err := protocols.Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	chs := gsession.NewSessionChans()
	defer func() {
		protocols.HandleError(ctx, sess, err, ws, chs)
	}()

	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": "monitor session"}})
		return
	}

	if sess = gsession.GetOnlineSessionById(sessionId); sess == nil {
		err = &myErrors.ApiError{Code: myErrors.ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	if sess.IsGuacd() {
		g.Go(func() error {
			return protocols.MonitGuacd(ctx, sess, chs, ws)
		})
	}

	key := fmt.Sprintf("%d-%s-%d", currentUser.GetUid(), sessionId, time.Now().Nanosecond())
	sess.Monitors.Store(key, ws)
	defer sess.Monitors.Delete(key)

	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				_, p, err := ws.ReadMessage()
				if err != nil {
					return err
				}
				if sess.IsGuacd() {
					chs.InChan <- p
				}
			}
		}
	})

	if err = g.Wait(); err != nil {
		logger.L().Error("monitor failed", zap.Error(err))
	}
}

func ConnectClose(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"perm": "close session"}})
		return
	}

	sessionService := service.NewSessionService()
	session, err := sessionService.GetOnlineSessionByID(ctx, ctx.Param("session_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "invalid session id"}})
		return
	}

	logger.L().Info("closing...", zap.String("sessionId", session.SessionId), zap.Int("type", session.SessionType))
	defer protocols.OfflineSession(ctx, session.SessionId, currentUser.GetUserName())

	session.Status = model.SESSIONSTATUS_OFFLINE
	session.ClosedAt = lo.ToPtr(time.Now())
	gsession.UpsertSession(session)

	ctx.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// DoConnect handles the connection setup process
func DoConnect(ctx *gin.Context, ws *websocket.Conn) (sess *gsession.Session, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	assetId, accountId := cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id"))
	asset, account, gateway, err := service.GetAAG(assetId, accountId)
	if err != nil {
		return
	}

	sess = gsession.NewSession(ctx)
	sess.Ws = ws
	sess.Session = &model.Session{
		SessionType: ctx.GetInt("sessionType"),
		SessionId:   uuid.New().String(),
		Uid:         currentUser.GetUid(),
		UserName:    currentUser.GetUserName(),
		AssetId:     assetId,
		Asset:       asset,
		AssetInfo:   fmt.Sprintf("%s(%s)", asset.Name, asset.Ip),
		AccountId:   accountId,
		AccountInfo: fmt.Sprintf("%s(%s)", account.Name, account.Account),
		GatewayId:   asset.GatewayId,
		GatewayInfo: lo.Ternary(asset.GatewayId == 0, "", fmt.Sprintf("%s(%s)", gateway.Name, gateway.Host)),
		Protocol:    ctx.Param("protocol"),
		Status:      model.SESSIONSTATUS_ONLINE,
		ShareId:     cast.ToInt(ctx.Value("shareId")),
	}
	if sess.ShareId != 0 {
		sess.ShareEnd, _ = ctx.Value("shareEnd").(time.Time)
		if shareErr, exists := ctx.Get("shareErr"); exists && shareErr != nil {
			if apiErr, ok := shareErr.(*myErrors.ApiError); ok {
				err = apiErr
				return
			}
		}
	}
	if !sess.IsGuacd() {
		w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
		sess.SshParser = gsession.NewParser(sess.SessionId, w, h)
		sess.SshParser.Protocol = sess.Protocol

		sessionService := service.NewSessionService()
		cmds, err := sessionService.GetSshParserCommands(ctx, []int(asset.AccessAuth.CmdIds))
		if err != nil {
			return sess, err
		}
		sess.SshParser.Cmds = cmds

		for _, c := range sess.SshParser.Cmds {
			if c.IsRe {
				c.Re, _ = regexp.Compile(c.Cmd)
			}
		}
		if sess.SshRecoder, err = gsession.NewAsciinema(sess.SessionId, w, h); err != nil {
			return sess, err
		}
	}
	if sess.SessionType == model.SESSIONTYPE_WEB {
		sess.ClientIp = ctx.ClientIP()
	} else if sess.SessionType == model.SESSIONTYPE_CLIENT {
		sess.ClientIp = ctx.RemoteIP()
	}

	if !protocols.CheckTime(asset.AccessAuth) {
		err = &myErrors.ApiError{Code: myErrors.ErrAccessTime}
		return
	}
	if ok, err := service.DefaultAuthService.HasAuthorization(ctx, sess); err != nil {
		err = &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}}
		return sess, err
	} else if !ok {
		err = &myErrors.ApiError{Code: myErrors.ErrUnauthorized, Data: map[string]any{"perm": "connect"}}
		return sess, err
	}

	switch strings.Split(sess.Protocol, ":")[0] {
	case "ssh":
		go protocols.ConnectSsh(ctx, sess, asset, account, gateway)
	case "redis", "mysql", "mongodb", "postgresql":
		go db.ConnectDB(sess, asset, account, gateway)
	case "telnet":
		go protocols.ConnectTelnet(ctx, sess, asset, account, gateway)
	case "vnc", "rdp":
		go protocols.ConnectGuacd(ctx, sess, asset, account, gateway)
	default:
		logger.L().Error("wrong protocol " + sess.Protocol)
	}

	if err = <-sess.Chans.ErrChan; err != nil {
		logger.L().Error("failed to connect", zap.Error(err))
		err = &myErrors.ApiError{Code: myErrors.ErrConnectServer, Data: map[string]any{"err": err}}
		return
	}

	gsession.GetOnlineSession().Store(sess.SessionId, sess)
	gsession.UpsertSession(sess)

	return
}

// HandleTerm handles terminal sessions
func HandleTerm(sess *gsession.Session, ctx *gin.Context) (err error) {
	defer func() {
		logger.L().Debug("defer HandleTerm", zap.String("sessionId", sess.SessionId))
		sess.SshParser.Close(sess.Prompt)
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if err = gsession.UpsertSession(sess); err != nil {
			logger.L().Error("offline session failed", zap.String("sessionId", sess.SessionId), zap.Error(err))
			return
		}
	}()
	chs := sess.Chans
	tk, tk1s, tk1m := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second), time.NewTicker(time.Minute)
	assetService := service.NewAssetService()
	sess.G.Go(func() error {
		return protocols.Read(sess)
	})
	sess.G.Go(func() (err error) {
		defer sess.Chans.Rin.Close()
		defer sess.Chans.Wout.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				protocols.Write(sess)
				return
			case <-chs.AwayChan:
				return
			case <-sess.IdleTk.C:
				msg := (&myErrors.ApiError{Code: myErrors.ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}).MessageWithCtx(ctx)
				protocols.WriteErrMsg(sess, msg)
				return &myErrors.ApiError{Code: myErrors.ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}
			case <-tk1m.C:
				asset, err := assetService.GetById(sess.Gctx, sess.AssetId)
				if err != nil {
					continue
				}
				if protocols.CheckTime(asset.AccessAuth) && (sess.ShareId == 0 || time.Now().Before(sess.ShareEnd)) {
					continue
				}
				return &myErrors.ApiError{Code: myErrors.ErrAccessTime}
			case closeBy := <-chs.CloseChan:
				msg := (&myErrors.ApiError{Code: myErrors.ErrAdminClose, Data: map[string]any{"admin": closeBy}}).MessageWithCtx(ctx)
				protocols.WriteErrMsg(sess, msg)
				logger.L().Info("closed by", zap.String("admin", closeBy))
				return &myErrors.ApiError{Code: myErrors.ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err = <-chs.ErrChan:
				protocols.WriteErrMsg(sess, err.Error())
				return
			case in := <-chs.InChan:
				if sess.SessionType == model.SESSIONTYPE_WEB {
					rt := in[0]
					msg := in[1:]
					switch rt {
					case '1':
						in = msg
					case '9':
						continue
					case 'w':
						wh := strings.Split(string(msg), ",")
						if len(wh) < 2 {
							continue
						}
						chs.WindowChan <- ssh.Window{
							Width:  cast.ToInt(wh[0]),
							Height: cast.ToInt(wh[1]),
						}
						continue
					}
				}
				if cmd, forbidden := sess.SshParser.AddInput(in); forbidden {
					protocols.WriteErrMsg(sess, fmt.Sprintf("%s is forbidden\n", cmd))
					sess.SshParser.AddInput(byteClearAll)
					chs.Win.Write(byteClearAll)
					continue
				}
				if _, err = chs.Win.Write(in); err != nil {
					return
				}
			case out := <-chs.OutChan:
				if _, err = chs.OutBuf.Write(out); err != nil {
					return
				}
				sess.SshParser.AddOutput(out)
			case <-tk.C:
				if err = protocols.Write(sess); err != nil {
					return
				}
			case <-tk1s.C:
				if sess.Ws == nil {
					continue
				}
				if err = sess.Ws.WriteMessage(websocket.TextMessage, nil); err != nil {
					return
				}
			}
		}
	})

	if err = sess.G.Wait(); err != nil {
		logger.L().Debug("handle term wait end", zap.String("id", sess.SessionId), zap.Error(err))
	}

	return
}
