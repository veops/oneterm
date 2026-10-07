package connector

import (
	"errors"
	"fmt"
	"net/http"
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
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	fileservice "github.com/veops/oneterm/internal/service/file"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
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
		protocols.HandleGuacd(sess, ctx)
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
	if sess.IsGuacd() {
		sess.Monitors.Store(key, ws)
	} else {
		monitor := protocols.NewMonitor(ws, ctx.Query("binary") == "true")
		defer monitor.Close()
		sess.Monitors.Store(key, monitor)
	}
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

	g.Wait()
	logger.L().Info("monitor exit", zap.String("sessionId", sess.SessionId))
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
	asset, account, gateway, err := repository.GetAAGMetadata(ctx.Request.Context(), assetId, accountId)
	if err != nil {
		return
	}

	sessionId := ctx.Query("session_id")
	if sessionId == "" {
		sessionId = uuid.New().String()
	}

	sess = gsession.NewSession(ctx.Request.Context())
	defer func() {
		if err != nil {
			CloseTerminalSession(sess)
		}
	}()
	sess.Ws = ws
	sess.BinaryOutput = ctx.Query("binary") == "true"
	sess.Session = &model.Session{
		SessionType: ctx.GetInt("sessionType"),
		SessionId:   sessionId,
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

		// Use V2 command analyzer instead of legacy method
		commandAnalyzer := service.NewCommandAnalyzer()
		cmds, err := commandAnalyzer.AnalyzeSessionCommands(ctx, sess)
		if err != nil {
			logger.L().Error("Failed to analyze session commands", zap.String("sessionId", sess.SessionId), zap.Error(err))
			return sess, err
		}
		sess.SshParser.Cmds = cmds

		if sess.SshRecoder, err = gsession.NewAsciinema(sess.SessionId, w, h); err != nil {
			return sess, err
		}
	}
	switch sess.SessionType {
	case model.SESSIONTYPE_WEB:
		sess.ClientIp = ctx.ClientIP()
	case model.SESSIONTYPE_CLIENT:
		sess.ClientIp = ctx.RemoteIP()
	}

	// V2 authorization check - determine required permissions based on protocol
	protocol := strings.Split(sess.Protocol, ":")[0]
	var requiredActions []model.AuthAction

	// All protocols need connect permission
	requiredActions = append(requiredActions, model.ActionConnect)

	// SSH protocol needs file permissions since it will initialize SFTP client
	if protocol == "ssh" {
		requiredActions = append(requiredActions, model.ActionFileUpload, model.ActionFileDownload)
	}

	// Web protocols need file download permission check
	if protocol == "http" || protocol == "https" {
		requiredActions = append(requiredActions, model.ActionFileDownload)
	}
	if protocol == "rdp" || protocol == "vnc" {
		requiredActions = append(requiredActions, model.ActionCopy, model.ActionPaste, model.ActionFileUpload, model.ActionFileDownload)
	}

	// RDP/VNC are handled separately in ConnectGuacd with their own batch permission check
	// but we still check connect permission here for consistency

	result, err := service.DefaultAuthService.HasAuthorizationV2(ctx, sess, requiredActions...)
	if err != nil {
		err = &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}}
		return sess, err
	}

	// Check connect permission (required for all protocols)
	if !result.IsAllowed(model.ActionConnect) {
		err = &myErrors.ApiError{Code: myErrors.ErrUnauthorized, Data: map[string]any{"perm": "connect"}}
		return sess, err
	}
	sess.PAMAuthorization = result.PAM
	if result.PAM != nil {
		if err = service.AdmitPAMConnection(ctx.Request.Context(), sess.SessionId, result.PAM); err != nil {
			if sess.IdleTk != nil {
				sess.IdleTk.Stop()
			}
			return sess, pamConnectionError(err)
		}
		defer func() {
			if err != nil {
				stopPAMConnection(sess)
				if sess.IdleTk != nil {
					sess.IdleTk.Stop()
				}
				releasePAMConnection(sess)
			}
		}()
		// A late dial result must not retain a sender after admission has expired.
		sess.Chans.ErrChan = make(chan error, 1)
		sess.SetPermissions(&model.AuthPermissions{Connect: true, Copy: result.IsAllowed(model.ActionCopy), Paste: result.IsAllowed(model.ActionPaste),
			FileUpload: result.IsAllowed(model.ActionFileUpload), FileDownload: result.IsAllowed(model.ActionFileDownload)})
	}
	if err = repository.ResolveAssetAccountCredential(ctx.Request.Context(), asset, account); err != nil {
		return sess, err
	}
	sess.AccountInfo = fmt.Sprintf("%s(%s)", account.Name, account.Account)
	if gateway.Id != 0 {
		if err = repository.ResolveCredential(ctx.Request.Context(), gateway); err != nil {
			return sess, err
		}
	}

	// Set permissions in session for protocol-specific usage
	if protocol == "http" || protocol == "https" {
		// For Web protocols, store all relevant permissions
		permissions := &model.AuthPermissions{
			Connect:      result.IsAllowed(model.ActionConnect),
			FileDownload: result.IsAllowed(model.ActionFileDownload),
			Copy:         result.IsAllowed(model.ActionCopy),
			Paste:        result.IsAllowed(model.ActionPaste),
			Share:        result.IsAllowed(model.ActionShare),
		}
		sess.SetPermissions(permissions)
	}

	switch protocol {
	case "ssh":
		go protocols.ConnectSsh(ctx, sess, asset, account, gateway)
	case "redis", "mysql", "mongodb", "postgresql":
		go db.ConnectDB(sess, asset, account, gateway)
	case "telnet":
		go protocols.ConnectTelnet(ctx, sess, asset, account, gateway)
	case "vnc", "rdp":
		go protocols.ConnectGuacd(ctx, sess, asset, account, gateway)
	case "http", "https":
		// Web assets are handled through separate web proxy API endpoints
		err = &myErrors.ApiError{Code: myErrors.ErrConnectServer, Data: map[string]any{"err": "Web assets should use web proxy API"}}
		sess.Chans.ErrChan <- err
		return
	default:
		return sess, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "unsupported protocol"}}
	}

	if sess.PAMAuthorization == nil {
		select {
		case err = <-sess.Chans.ErrChan:
		case <-sess.Gctx.Done():
			err = sess.Gctx.Err()
		}
	} else {
		err = waitPAMConnection(ctx.Request.Context(), sess)
	}
	if err != nil {
		logger.L().Error("failed to connect", zap.Error(err))
		var admissionErr *myErrors.ApiError
		if sess.PAMAuthorization != nil && errors.As(err, &admissionErr) {
			return sess, admissionErr
		}
		err = &myErrors.ApiError{Code: myErrors.ErrConnectServer, Data: map[string]any{"err": err}}
		return
	}
	if sess.PAMAuthorization != nil {
		if err = service.ActivatePAMConnection(ctx.Request.Context(), sess.PAMAuthorization); err != nil {
			return sess, pamConnectionError(err)
		}
		sess.G.Go(func() error {
			err := service.WatchPAMConnection(sess.Gctx, sess.PAMAuthorization)
			stopPAMConnection(sess)
			releasePAMConnection(sess)
			return err
		})
	}

	gsession.GetOnlineSession().Store(sess.SessionId, sess)
	gsession.UpsertSession(sess)
	if ws != nil && !sess.IsGuacd() && sess.BinaryOutput && ctx.Query("flow") == "true" {
		if err = sess.StartOutputFlow(); err != nil {
			return sess, err
		}
	}

	return
}

// HandleTerm handles terminal sessions
func HandleTerm(sess *gsession.Session, ctx *gin.Context) (err error) {
	defer CloseTerminalSession(sess)
	chs := sess.Chans
	tk, tk1s := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second)
	defer tk.Stop()
	defer tk1s.Stop()
	sess.G.Go(func() error { return WatchTerminalSession(sess, ctx) })
	sess.G.Go(func() error {
		return protocols.Read(sess)
	})
	sess.G.Go(func() (err error) {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return nil
			case in := <-chs.InChan:
				if sess.SessionType == model.SESSIONTYPE_WEB {
					var window ssh.Window
					in, window = protocols.TerminalMessage(in)
					if window.Width > 0 {
						chs.Resize(window)
					}
					if len(in) == 0 {
						continue
					}
				}
				if cmd, forbidden := sess.SshParser.AddInput(in); forbidden {
					protocols.WriteErrMsg(sess, fmt.Sprintf("%s is forbidden\n", cmd), true)
					sess.SshParser.AddInput(byteClearAll)
					chs.Win.Write(byteClearAll)
					continue
				}
				if _, err = chs.Win.Write(in); err != nil {
					if sess.Gctx.Err() != nil {
						return nil
					}
					return
				}
			}
		}
	})
	sess.G.Go(func() (err error) {
		defer sess.Stop()
		defer sess.Chans.Rin.Close()
		defer sess.Chans.Wout.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				protocols.Write(sess)
				return
			case <-chs.AwayChan:
				return flushTerminalOutput(sess)
			case out := <-chs.OutChan:
				if _, err = chs.OutBuf.Write(out); err != nil {
					return
				}
				sess.SshParser.AddOutput(out)
				if chs.OutBuf.Len() >= 32768 {
					if err = protocols.Write(sess); err != nil {
						return err
					}
				}
			case <-tk.C:
				if err = protocols.Write(sess); err != nil {
					return
				}
			case <-tk1s.C:
				if sess.Ws == nil {
					continue
				}
				if err = sess.WriteWebsocket(websocket.TextMessage, nil); err != nil {
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

func flushTerminalOutput(sess *gsession.Session) error {
	for {
		select {
		case out := <-sess.Chans.OutChan:
			sess.Chans.OutBuf.Write(out)
			sess.SshParser.AddOutput(out)
		default:
			return protocols.Write(sess)
		}
	}
}

func WatchTerminalSession(sess *gsession.Session, ctx *gin.Context) error {
	return protocols.WatchTerminalSession(sess, ctx)
}

func CloseTerminalSession(sess *gsession.Session) {
	if sess == nil || sess.Session == nil {
		return
	}
	sess.Stop()
	sess.StopIdle()
	gsession.GetOnlineSession().Delete(sess.SessionId)
	if fileservice.DefaultFileService != nil {
		fileservice.DefaultFileService.CloseSessionFileClient(sess.SessionId)
	}
	sess.ClearSSHClient()
	tunneling.CloseTunnels(sess.SessionId)
	if sess.SshParser != nil {
		sess.SshParser.Close(sess.Prompt)
	}
	if sess.SshRecoder != nil {
		if err := sess.SshRecoder.Close(); err != nil {
			logger.L().Error("close session recording failed", zap.Error(err))
		}
	}
	sess.Status, sess.ClosedAt = model.SESSIONSTATUS_OFFLINE, lo.ToPtr(time.Now())
	if sess.Id > 0 {
		if err := gsession.UpsertSession(sess); err != nil {
			logger.L().Error("close session failed", zap.Error(err))
		}
	}
}
