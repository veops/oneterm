package controller

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/guacd"
	mysql "github.com/veops/oneterm/db"
	ggateway "github.com/veops/oneterm/gateway"
	myi18n "github.com/veops/oneterm/i18n"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	gsession "github.com/veops/oneterm/session"
	"github.com/veops/oneterm/util"
)

var (
	Upgrader = websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	clear = []byte("\x15\r")
)

func read(sess *gsession.Session) error {
	chs := sess.Chans
	for {
		select {
		case <-sess.Gctx.Done():
			return nil
		default:
			if sess.SessionType == model.SESSIONTYPE_WEB {
				t, msg, err := sess.Ws.ReadMessage()
				if err != nil {
					return err
				}
				if len(msg) <= 0 {
					continue
				}
				switch t {
				case websocket.TextMessage:
					chs.InChan <- msg
					if (sess.IsSsh() && len(msg) > 0 && msg[0] != '9') || (!sess.IsSsh() && guacd.IsActive(msg)) {
						sess.SetIdle()
					}
				}
			} else if sess.SessionType == model.SESSIONTYPE_CLIENT {
				sess.SetIdle()
				chs.InChan <- sess.CliRw.Read()
			}
		}
	}
}

func write(sess *gsession.Session) {
	chs := sess.Chans
	out := chs.OutBuf.Bytes()

	if sess.SessionType == model.SESSIONTYPE_WEB && sess.Ws != nil {
		if len(out) > 0 || !strings.Contains(sess.Protocol, "ssh") {
			sess.Ws.WriteMessage(websocket.TextMessage, out)
		}
	} else if sess.SessionType == model.SESSIONTYPE_CLIENT && len(out) > 0 {
		sess.CliRw.Write(out)
	}

	if sess.SshRecoder != nil && len(out) > 0 && strings.Contains(sess.Protocol, "ssh") {
		sess.SshRecoder.Write(out)
	}

	writeToMonitors(sess.Monitors, out)
	chs.OutBuf.Reset()
}

func writeErrMsg(sess *gsession.Session, msg string) {
	chs := sess.Chans
	out := []byte(fmt.Sprintf("\r\n \033[31m %s \x1b[0m", msg))
	chs.OutBuf.Write(out)
	write(sess)
}

func HandleSsh(sess *gsession.Session) (err error) {
	defer func() {
		sess.SshParser.WriteDb()
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if err = gsession.UpsertSession(sess); err != nil {
			logger.L().Error("offline ssh session failed", zap.Error(err))
			return
		}
	}()
	chs := sess.Chans
	tk, tk1s, tk1m := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second), time.NewTicker(time.Minute)
	sess.G.Go(func() error {
		return read(sess)
	})
	sess.G.Go(func() error {
		asset := &model.Asset{}
		defer sess.Chans.Rin.Close()
		defer sess.Chans.Wout.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				write(sess)
				return nil
			case <-sess.IdleTk.C:
				writeErrMsg(sess, "idle timeout\n\n")
				return &ApiError{Code: ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}
			case <-tk1m.C:
				if mysql.DB.Model(asset).Where("id = ?", sess.AssetId).First(asset).Error != nil {
					continue
				}
				if checkTime(asset.AccessAuth) && (sess.ShareId == 0 || time.Now().Before(sess.ShareEnd)) {
					continue
				}
				return &ApiError{Code: ErrAccessTime}
			case closeBy := <-chs.CloseChan:
				writeErrMsg(sess, "closed by admin\n\n")
				logger.L().Info("closed by", zap.String("admin", closeBy))
				return &ApiError{Code: ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err := <-chs.ErrChan:
				writeErrMsg(sess, err.Error())
				return err
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
					}
				}
				if cmd, forbidden := sess.SshParser.AddInput(in); forbidden {
					writeErrMsg(sess, fmt.Sprintf("%s is forbidden\n", cmd))
					sess.SshParser.AddInput(clear)
					chs.Win.Write(clear)
					continue
				}
				chs.Win.Write(in)
			case out := <-chs.OutChan:
				chs.OutBuf.Write(out)
				sess.SshParser.AddOutput(out)
			case <-tk.C:
				write(sess)
			case <-tk1s.C:
				if sess.Ws != nil {
					sess.Ws.WriteMessage(websocket.TextMessage, nil)
				}
			}
		}
	})

	if err = sess.G.Wait(); err != nil {
		logger.L().Debug("sess wait end ssh", zap.String("id", sess.SessionId), zap.Error(err))
	}

	return
}

func handleGuacd(sess *gsession.Session) (err error) {
	defer func() {
		sess.GuacdTunnel.Disconnect()
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if err = gsession.UpsertSession(sess); err != nil {
			logger.L().Error("offline ssh session failed", zap.Error(err))
			return
		}
	}()
	chs := sess.Chans
	tk := time.NewTicker(time.Minute)
	asset := &model.Asset{}
	sess.G.Go(func() error {
		return read(sess)
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-sess.IdleTk.C:
				return &ApiError{Code: ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}
			case <-tk.C:
				if mysql.DB.Model(asset).Where("id = ?", sess.AssetId).First(asset).Error != nil {
					continue
				}
				if checkTime(asset.AccessAuth) && (sess.ShareId == 0 || time.Now().Before(sess.ShareEnd)) {
					continue
				}
				return &ApiError{Code: ErrAccessTime}
			case closeBy := <-chs.CloseChan:
				return &ApiError{Code: ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err := <-chs.ErrChan:
				return err
			case out := <-chs.OutChan:
				sess.Ws.WriteMessage(websocket.TextMessage, out)
			}
		}
	})

	if err = sess.G.Wait(); err != nil {
		logger.L().Debug("sess wait end guacd", zap.String("id", sess.SessionId), zap.Error(err))
	}

	return
}

func writeToMonitors(monitors *sync.Map, out []byte) {
	monitors.Range(func(key, value any) bool {
		ws, ok := value.(*websocket.Conn)
		if !ok || ws == nil {
			return true
		}
		ws.WriteMessage(websocket.TextMessage, out)
		return true
	})
}

func DoConnect(ctx *gin.Context, ws *websocket.Conn) (sess *gsession.Session, err error) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	assetId, accountId := cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id"))
	asset, account, gateway, err := util.GetAAG(assetId, accountId)
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
		if err, _ = ctx.Value("shareErr").(error); err != nil {
			return
		}
	}
	if sess.IsSsh() {
		w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
		sess.SshParser = gsession.NewParser(sess.SessionId, w, h)
		if err = mysql.DB.Model(sess.SshParser.Cmds).Where("id IN ? AND enable=?", []int(asset.AccessAuth.CmdIds), true).
			Find(&sess.SshParser.Cmds).Error; err != nil {
			return
		}
		for _, c := range sess.SshParser.Cmds {
			if c.IsRe {
				c.Re, _ = regexp.Compile(c.Cmd)
			}
		}
		if sess.SshRecoder, err = gsession.NewAsciinema(sess.SessionId, w, h); err != nil {
			return
		}
	}
	if sess.SessionType == model.SESSIONTYPE_WEB {
		sess.ClientIp = ctx.ClientIP()
	} else if sess.SessionType == model.SESSIONTYPE_CLIENT {
		sess.ClientIp = ctx.RemoteIP()
	}

	if !checkTime(asset.AccessAuth) {
		err = &ApiError{Code: ErrAccessTime}
		return
	}
	if !hasAuthorization(ctx, sess) {
		err = &ApiError{Code: ErrUnauthorized}
		return
	}

	switch strings.Split(sess.Protocol, ":")[0] {
	case "ssh":
		go connectSsh(ctx, sess, asset, account, gateway)
	case "vnc", "rdp":
		go connectGuacd(ctx, sess, asset, account, gateway)
	default:
		logger.L().Error("wrong protocol " + sess.Protocol)
	}

	if err = <-sess.Chans.ErrChan; err != nil {
		logger.L().Error("failed to connect", zap.Error(err))
		err = &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": err}}
		return
	}

	gsession.GetOnlineSession().Store(sess.SessionId, sess)
	gsession.UpsertSession(sess)

	return
}

func connectSsh(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
	chs := sess.Chans
	defer func() {
		ggateway.GetGatewayManager().Close(sess.SessionId)
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	ip, port, err := util.Proxy(sess.SessionId, "ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := util.GetAuth(account)
	if err != nil {
		return
	}

	sshCli, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &gossh.ClientConfig{
		User:            account.Account,
		Auth:            []gossh.AuthMethod{auth},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		logger.L().Error("ssh dial failed", zap.Error(err))
		return
	}

	sshSess, err := sshCli.NewSession()
	if err != nil {
		logger.L().Error("ssh session create failed", zap.Error(err))
		return
	}
	defer sshSess.Close()

	sshSess.Stdin = chs.Rin
	sshSess.Stdout = chs.Wout
	sshSess.Stderr = chs.Wout

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err = sshSess.RequestPty("xterm", h, w, modes); err != nil {
		logger.L().Error("ssh request pty failed", zap.Error(err))
		return
	}
	if err = sshSess.Shell(); err != nil {
		logger.L().Error("ssh start shell failed", zap.Error(err))
		return
	}

	sess.G.Go(func() error {
		err = sshSess.Wait()
		return fmt.Errorf("ssh session wait end %w", err)
	})

	chs.ErrChan <- err

	sess.G.Go(func() error {
		buf := bufio.NewReader(chs.Rout)
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				rn, size, err := buf.ReadRune()
				if err != nil {
					return err
				}
				if size <= 0 || rn == utf8.RuneError {
					continue
				}
				p := make([]byte, utf8.RuneLen(rn))
				utf8.EncodeRune(p, rn)
				chs.OutChan <- p
			}
		}
	})
	sess.G.Go(func() error {
		defer sshSess.Close()
		defer sess.Chans.Rout.Close()
		defer sess.Chans.Win.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case window := <-chs.WindowChan:
				if err := sshSess.WindowChange(window.Height, window.Width); err != nil {
					logger.L().Warn("reset window size failed", zap.Error(err))
					continue
				}
				sess.SshRecoder.Resize(window.Width, window.Height)
				sess.SshParser.Resize(window.Width, window.Height)
			}
		}
	})

	sess.G.Wait()

	return
}

func connectGuacd(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	t, err := guacd.NewTunnel("", sess.SessionId, w, h, dpi, sess.Protocol, asset, account, gateway)
	if err != nil {
		logger.L().Error("guacd tunnel failed", zap.Error(err))
		return
	}
	defer t.Close()

	sess.ConnectionId = t.ConnectionId
	sess.GuacdTunnel = t

	chs.ErrChan <- nil

	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if err != nil {
					return err
				}
				if len(p) <= 0 {
					continue
				}

				chs.OutChan <- p
			}
		}
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})

	sess.G.Wait()

	return
}

// Connect godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Param		w	query		int	false	"width"
//	@Param		h	query		int	false	"height"
//	@Param		dpi	query		int	false	"dpi"
//	@Success	200	{object}	HttpResponse{}
//	@Router		/connect/:asset_id/:account_id/:protocol [get]
func (c *Controller) Connect(ctx *gin.Context) {
	ctx.Set("sessionType", model.SESSIONTYPE_WEB)

	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	var sess *gsession.Session
	defer func() {
		handleError(ctx, sess, err, ws, nil)
	}()

	sess, err = DoConnect(ctx, ws)
	if err != nil {
		return
	}

	if sess.IsSsh() {
		HandleSsh(sess)
	} else {
		handleGuacd(sess)
	}
}

// ConnectMonitor godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Router		/connect/monitor/:session_id [get]
func (c *Controller) ConnectMonitor(ctx *gin.Context) {

	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sessionId := ctx.Param("session_id")
	var sess *gsession.Session
	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	chs := gsession.NewSessionChans()
	defer func() {
		handleError(ctx, sess, err, ws, chs)
	}()

	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": "monitor session"}})
		return
	}

	if sess = gsession.GetOnlineSessionById(sessionId); sess == nil {
		err = &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	if !sess.IsSsh() {
		g.Go(func() error {
			return monitGuacd(ctx, sess, chs, ws)
		})
	}

	key := fmt.Sprintf("%d-%s-%d", currentUser.Uid, sessionId, time.Now().Nanosecond())
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
				if !sess.IsSsh() {
					chs.InChan <- p
				}
			}
		}
	})

	if err = g.Wait(); err != nil {
		logger.L().Error("monitor failed", zap.Error(err))
	}
}

func monitGuacd(ctx *gin.Context, sess *gsession.Session, chs *gsession.SessionChans, ws *websocket.Conn) (err error) {
	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	defer func() {
		chs.ErrChan <- err
	}()

	t, err := guacd.NewTunnel(sess.ConnectionId, "", w, h, dpi, ":", nil, nil, nil)
	if err != nil {
		logger.L().Error("guacd tunnel failed", zap.Error(err))
		return
	}
	defer t.Disconnect()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if err != nil {
					logger.L().Debug("read instruction failed", zap.Error(err))
					return err
				}
				if len(p) <= 0 {
					continue
				}
				chs.OutChan <- p
			}
		}
	})
	g.Go(func() error {
		for {
			select {
			case <-sess.Chans.AwayChan:
				err := fmt.Errorf("monitored session closed")
				ws.WriteMessage(websocket.TextMessage, guacd.NewInstruction("disconnect", err.Error()).Bytes())
				return err
			case err := <-chs.ErrChan:
				return err
			case out := <-chs.OutChan:
				ws.WriteMessage(websocket.TextMessage, out)
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})
	if err = g.Wait(); err != nil {
		logger.L().Warn("monit guacd stopped", zap.Error(err))
	}

	return
}

// ConnectClose godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Router		/connect/close/:session_id [post]
func (c *Controller) ConnectClose(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": "close session"}})
		return
	}

	session := &gsession.Session{}
	err := mysql.DB.
		Model(session).
		Where("session_id = ?", ctx.Param("session_id")).
		Where("status = ?", model.SESSIONSTATUS_ONLINE).
		First(session).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.JSON(http.StatusOK, defaultHttpResponse)
		return
	}
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": "invalid session id"}})
		return
	}

	logger.L().Info("closing...", zap.String("sessionId", session.SessionId), zap.Int("type", session.SessionType))
	defer offlineSession(ctx, session.SessionId, currentUser.GetUserName())

	session.Status = model.SESSIONSTATUS_OFFLINE
	session.ClosedAt = lo.ToPtr(time.Now())
	gsession.UpsertSession(session)

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

func offlineSession(ctx *gin.Context, sessionId string, closer string) {
	logger.L().Debug("offline", zap.String("session_id", sessionId), zap.String("closer", closer))
	defer gsession.GetOnlineSession().Delete(sessionId)
	session := gsession.GetOnlineSessionById(sessionId)
	if session == nil {
		return
	}
	if closer != "" && session.Chans != nil {
		select {
		case session.Chans.CloseChan <- closer:
			break
		case <-time.After(time.Second):
			break
		}

	}
	session.Monitors.Range(func(key, value any) bool {
		ws, ok := value.(*websocket.Conn)
		if ok && ws != nil {
			lang := ctx.PostForm("lang")
			accept := ctx.GetHeader("Accept-Language")
			localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
			cfg := &i18n.LocalizeConfig{
				TemplateData:   map[string]any{"sessionId": sessionId},
				DefaultMessage: myi18n.MsgSessionEnd,
			}
			msg, _ := localizer.Localize(cfg)
			ws.WriteMessage(websocket.TextMessage, []byte(msg))
			ws.Close()
		}
		return true
	})
}

func checkTime(data model.AccessAuth) bool {
	now := time.Now()
	in := true
	if (data.Start != nil && now.Before(*data.Start)) || (data.End != nil && now.After(*data.End)) {
		in = false
	}
	if !in {
		return false
	}
	in = false
	has := false
	week, hm := now.Weekday(), now.Format("15:04")
	for _, r := range data.Ranges {
		has = has || len(r.Times) > 0
		if (r.Week+1)%7 == int(week) {
			for _, str := range r.Times {
				ss := strings.Split(str, "~")
				in = in || (len(ss) >= 2 && hm >= ss[0] && hm <= ss[1])
			}
		}
	}
	return !has || in == data.Allow
}

func handleError(ctx *gin.Context, sess *gsession.Session, err error, ws *websocket.Conn, chs *gsession.SessionChans) {
	defer func() {
		if chs == nil {
			close(sess.Chans.AwayChan)
		} else {
			close(chs.AwayChan)
		}
	}()

	if err == nil {
		return
	}
	logger.L().Debug("", zap.String("session_id", sess.SessionId), zap.Error(err))
	ae, ok := err.(*ApiError)
	if sess.IsSsh() {
		writeErrMsg(sess, lo.Ternary(ok, ae.MessageWithCtx(ctx), err.Error()))
	} else {
		ws.WriteMessage(websocket.TextMessage, guacd.NewInstruction("error", lo.Ternary(ok, (ae).MessageBase64(ctx), err.Error()), cast.ToString(ErrAdminClose)).Bytes())
	}
}
