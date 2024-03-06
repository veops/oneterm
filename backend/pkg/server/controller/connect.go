package controller

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/conf"
	myi18n "github.com/veops/oneterm/pkg/i18n"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
	gsession "github.com/veops/oneterm/pkg/server/global/session"
	"github.com/veops/oneterm/pkg/server/guacd"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
	"github.com/veops/oneterm/pkg/util"
)

var (
	Upgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// Connect godoc
//
//	@Tags		connect
//	@Param		w	query		int	false	"width"
//	@Param		h	query		int	false	"height"
//	@Param		dpi	query		int	false	"dpi"
//	@Success	200	{object}	HttpResponse
//	@Param		session_id	path		int	true	"session id"
//	@Router		/connect/:session_id [get]
func (c *Controller) Connecting(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	var session *gsession.Session

	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	defer func() {
		handleError(ctx, session, err, ws)
	}()

	if session, err = loadOnlineSessionById(sessionId, false); err != nil {
		return
	}
	session.Connected.Store(true)
	if session.IsSsh() {
		err = handleSsh(ctx, ws, session)
	} else {
		err = handleGuacd(ctx, ws, session)
	}
}

func handleSsh(ctx *gin.Context, ws *websocket.Conn, session *gsession.Session) (err error) {
	chs := session.Chans
	chs.WindowChan <- fmt.Sprintf("%s,%s,%s", ctx.Query("w"), ctx.Query("h"), ctx.Query("dpi"))
	tk, tk1s := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return readWsMsg(gctx, ws, session)
	})
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case closeBy := <-chs.CloseChan:
				out := []byte("\r\n \033[31m closed by admin")
				ws.WriteMessage(websocket.TextMessage, out)
				writeToMonitors(session.Monitors, out)
				err := fmt.Errorf("colse by admin %s", closeBy)
				logger.L.Warn(err.Error())
				return err
			case err := <-chs.ErrChan:
				logger.L.Error("server disconnected", zap.Error(err))
				return err
			case in := <-chs.InChan:
				rt := in[0]
				msg := in[1:]
				switch rt {
				case '1':
					chs.Win.Write(msg)
				case '9':
					continue
				case 'w':
					chs.WindowChan <- string(msg)
				}
			case out := <-chs.OutChan:
				chs.Buf.Write(out)
			case <-tk.C:
				sendSshMsg(ws, session, chs)
			case <-tk1s.C:
				ws.WriteMessage(websocket.TextMessage, nil)
				writeToMonitors(session.Monitors, nil)
			}
		}
	})
	err = g.Wait()
	return
}

func handleGuacd(ctx *gin.Context, ws *websocket.Conn, session *gsession.Session) (err error) {
	defer session.GuacdTunnel.Disconnect()
	chs := session.Chans
	g, gctx := errgroup.WithContext(ctx)
	session.IdleTimout = idleTime()
	session.IdleTk = time.NewTicker(session.IdleTimout)
	tk := time.NewTicker(time.Minute)
	asset := &model.Asset{}
	g.Go(func() error {
		return readWsMsg(gctx, ws, session)
	})
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case closeBy := <-chs.CloseChan:
				return &ApiError{Code: ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err := <-chs.ErrChan:
				logger.L.Error("disconnected", zap.Error(err))
				return err
			case <-tk.C:
				if mysql.DB.Model(asset).Where("id = ?", session.AssetId).First(asset).Error != nil {
					continue
				}
				if checkTime(asset.AccessAuth) {
					continue
				}
				return &ApiError{Code: ErrAccessTime}
			case <-session.IdleTk.C:
				return &ApiError{Code: ErrIdleTimeout, Data: map[string]any{"second": int64(session.IdleTimout.Seconds())}}
			case out := <-chs.OutChan:
				ws.WriteMessage(websocket.TextMessage, out)
			}
		}
	})
	err = g.Wait()
	return
}

func sendSshMsg(ws *websocket.Conn, session *gsession.Session, chs *gsession.SessionChans) {
	out := chs.Buf.Bytes()
	if len(out) <= 0 {
		return
	}
	if ws != nil {
		ws.WriteMessage(websocket.TextMessage, out)
	}
	if session != nil && session.IsSsh() {
		writeToMonitors(session.Monitors, out)
	}
	chs.Buf.Reset()
}

// Connect godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Param		w	query		int	false	"width"
//	@Param		h	query		int	false	"height"
//	@Param		dpi	query		int	false	"dpi"
//	@Success	200			{object}	HttpResponse{data=gsession.Session}
//	@Router		/connect/:asset_id/:account_id/:protocol [post]
func (c *Controller) Connect(ctx *gin.Context) {
	protocol, chs := ctx.Param("protocol"), makeChans()
	sessionId, resp := "", &gsession.ServerResp{}

	switch strings.Split(protocol, ":")[0] {
	case "ssh":
		go connectSsh(ctx, newSshReq(ctx, model.SESSIONACTION_NEW), chs)
	case "vnc", "rdp":
		currentUser, _ := acl.GetSessionFromCtx(ctx)
		asset := &model.Asset{}
		if err := mysql.DB.Model(&asset).Where("id = ?", ctx.Param("asset_id")).First(asset).Error; err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
			return
		}
		if !checkAuthorization(currentUser, asset, cast.ToInt(ctx.Param("account_id"))) {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrLogin, Data: map[string]any{"err": fmt.Errorf("invalid authorization")}})
			return
		}
		if !checkTime(asset.AccessAuth) {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrAccessTime, Data: map[string]any{"err": fmt.Errorf("invalid authorization")}})
			return
		}
		go connectGuacd(ctx, asset, protocol, chs)
	default:
		logger.L.Error("wrong protocol " + protocol)
	}

	if err := <-chs.ErrChan; err != nil {
		logger.L.Error("failed to connect", zap.Error(err))
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": err}})
		return
	}
	resp = <-chs.RespChan
	if resp.Code != 0 {
		logger.L.Error("failed to connect", zap.Any("resp", *resp))
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": resp.Message}})
		return
	}
	sessionId = resp.SessionId
	v, ok := gsession.GetOnlineSession().Load(sessionId)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "cannot find in sync map"}})
		return
	}
	session, ok := v.(*gsession.Session)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "invalid type"}})
		return
	}
	session.Chans = chs

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(session))
	// go connectable.CheckUpdate(cast.ToInt(ctx.Param("asset_id")))
}

func readWsMsg(ctx context.Context, ws *websocket.Conn, session *gsession.Session) error {
	chs := session.Chans
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			t, msg, err := ws.ReadMessage()
			if err != nil {
				return err
			}
			if len(msg) <= 0 {
				logger.L.Warn("websocket msg length is zero")
				continue
			}
			switch t {
			case websocket.TextMessage:
				chs.InChan <- msg
				if !session.IsSsh() && guacd.IsActive(msg) {
					session.IdleTk.Reset(session.IdleTimout)
				}
			}
		}
	}
}

func newSshReq(ctx *gin.Context, action int) *gsession.SshReq {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	return &gsession.SshReq{
		Uid:            currentUser.GetUid(),
		UserName:       currentUser.GetUserName(),
		Cookie:         ctx.GetHeader("Cookie"),
		AcceptLanguage: ctx.GetHeader("Accept-Language"),
		ClientIp:       ctx.ClientIP(),
		AssetId:        cast.ToInt(ctx.Param("asset_id")),
		AccountId:      cast.ToInt(ctx.Param("account_id")),
		Protocol:       ctx.Param("protocol"),
		Action:         action,
		SessionId:      ctx.Param("session_id"),
	}
}

func makeChans() *gsession.SessionChans {
	rin, win := io.Pipe()
	return &gsession.SessionChans{
		Rin:        rin,
		Win:        win,
		ErrChan:    make(chan error),
		RespChan:   make(chan *gsession.ServerResp),
		InChan:     make(chan []byte),
		OutChan:    make(chan []byte),
		Buf:        &bytes.Buffer{},
		WindowChan: make(chan string),
		AwayChan:   make(chan struct{}),
		CloseChan:  make(chan string),
	}
}

func connectSsh(ctx *gin.Context, req *gsession.SshReq, chs *gsession.SessionChans) (err error) {
	w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
	defer func() {
		chs.ErrChan <- err
	}()

	cfg := &ssh.ClientConfig{
		User: conf.Cfg.SshServer.Account,
		Auth: []ssh.AuthMethod{
			ssh.Password(conf.Cfg.SshServer.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", conf.Cfg.SshServer.Ip, conf.Cfg.SshServer.Port), cfg)
	if err != nil {
		logger.L.Error("ssh tcp dail failed", zap.Error(err))
		return
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		logger.L.Error("ssh session create failed", zap.Error(err))
		return
	}
	defer sess.Close()

	rout, wout := io.Pipe()
	sess.Stdout = wout
	sess.Stderr = wout
	sess.Stdin = chs.Rin

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err = sess.RequestPty("xterm", h, w, modes); err != nil {
		logger.L.Error("ssh request pty failed", zap.Error(err))
		return
	}
	if err = sess.Shell(); err != nil {
		logger.L.Error("ssh start shell failed", zap.Error(err))
		return
	}

	bs, err := json.Marshal(req)
	if err != nil {
		logger.L.Error("ssh req marshal failed", zap.Error(err))
		return
	}
	if _, err = chs.Win.Write(append(bs, '\r')); err != nil {
		logger.L.Error("ssh req", zap.Error(err), zap.String("req content", string(bs)))
		return
	}

	buf := bufio.NewReader(rout)

	line, err := buf.ReadBytes('\r')
	if err != nil {
		logger.L.Error("ssh read bytes failed", zap.Error(err))
		return
	}
	resp := &gsession.ServerResp{}
	if err = json.Unmarshal([]byte(line)[0:len(line)-1], resp); err != nil {
		logger.L.Error("ssh resp", zap.Error(err), zap.String("resp content", string(line)))
		return
	}

	chs.ErrChan <- nil
	chs.RespChan <- resp

	g, gctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return sess.Wait()
	})
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				rn, size, err := buf.ReadRune()
				if isCtxDone(gctx) {
					return nil
				}
				if err != nil {
					logger.L.Debug("buf ReadRune failed", zap.Error(err))
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
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case s := <-chs.WindowChan:
				wh := strings.Split(s, ",")
				if len(wh) < 2 {
					continue
				}
				w = cast.ToInt(wh[0])
				h = cast.ToInt(wh[1])
				if w <= 0 || h <= 0 {
					continue
				}
				if err := sess.WindowChange(h, w); err != nil {
					logger.L.Warn("reset window size failed", zap.Error(err))
				}
			}
		}
	})
	if err = g.Wait(); err != nil {
		logger.L.Warn("doSsh stopped", zap.Error(err))
	}

	return
}

func connectGuacd(ctx *gin.Context, asset *model.Asset, protocol string, chs *gsession.SessionChans) {
	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	var err error
	defer func() {
		chs.ErrChan <- err
	}()

	account, gateway := &model.Account{}, &model.Gateway{}
	if err = mysql.DB.Model(&account).Where("id = ?", ctx.Param("account_id")).First(account).Error; err != nil {
		logger.L.Error("find account failed", zap.Error(err))
		return
	}
	if asset.GatewayId != 0 {
		if err = mysql.DB.Model(&gateway).Where("id = ?", asset.GatewayId).First(gateway).Error; err != nil {
			logger.L.Error("find gateway failed", zap.Error(err))
			return
		}
		gateway.Password = util.DecryptAES(gateway.Password)
		gateway.Pk = util.DecryptAES(gateway.Pk)
		gateway.Phrase = util.DecryptAES(gateway.Phrase)
	}

	t, err := guacd.NewTunnel("", w, h, dpi, protocol, asset, account, gateway)
	if err != nil {
		logger.L.Error("guacd tunnel failed", zap.Error(err))
		return
	}

	session := newGuacdSession(ctx, t.ConnectionId, t.SessionId, asset, account, gateway)
	if err = gsession.HandleUpsertSession(ctx, session); err != nil {
		return
	}
	session.GuacdTunnel = t

	resp := &gsession.ServerResp{
		Code:      lo.Ternary(err == nil, 0, -1),
		Message:   lo.TernaryF(err == nil, func() string { return "" }, func() string { return err.Error() }),
		SessionId: t.SessionId,
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
	}

	chs.ErrChan <- nil
	chs.RespChan <- resp

	g, gctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-time.After(time.Minute):
				if !session.Connected.Load() {
					session.Chans.AwayChan <- struct{}{}
				}
				return nil
			}
		}
	})
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if isCtxDone(gctx) {
					return nil
				}
				if err != nil {
					logger.L.Debug("read instruction failed", zap.Error(err))
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
		defer func() {
			t.Disconnect()
			session.Status = model.SESSIONSTATUS_OFFLINE
			session.ClosedAt = lo.ToPtr(time.Now())
			if err = gsession.HandleUpsertSession(ctx, session); err != nil {
				logger.L.Error("offline guacd session failed", zap.Error(err))
				return
			}
		}()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})
	if err = g.Wait(); err != nil {
		logger.L.Warn("doGuacd stopped", zap.Error(err))
	}
}

func newGuacdSession(ctx *gin.Context, connectionId, sessionId string, asset *model.Asset, account *model.Account, gateway *model.Gateway) *gsession.Session {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	return &gsession.Session{
		Session: &model.Session{
			SessionType: model.SESSIONTYPE_WEB,
			SessionId:   sessionId,
			Uid:         currentUser.GetUid(),
			UserName:    currentUser.GetUserName(),
			AssetId:     asset.Id,
			AssetInfo:   fmt.Sprintf("%s(%s)", asset.Name, asset.Ip),
			AccountId:   account.Id,
			AccountInfo: fmt.Sprintf("%s(%s)", account.Name, account.Account),
			GatewayId:   gateway.Id,
			GatewayInfo: lo.Ternary(gateway.Id == 0, "", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port)),
			ClientIp:    ctx.ClientIP(),
			Protocol:    ctx.Param("protocol"),
			Status:      model.SESSIONSTATUS_ONLINE,
		},
		ConnectionId: connectionId,
	}
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

// ConnectMonitor godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Router		/connect/monitor/:session_id [get]
func (c *Controller) ConnectMonitor(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sessionId := ctx.Param("session_id")
	var session *gsession.Session
	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	defer func() {
		handleError(ctx, session, err, ws)
	}()

	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": "monitor session"}})
		return
	}

	if session, err = loadOnlineSessionById(sessionId, true); err != nil {
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	chs := makeChans()
	switch session.SessionType {
	case model.SESSIONTYPE_WEB:
		if !session.IsSsh() {
			chs = makeChans()
			g.Go(func() error {
				return monitGuacd(ctx, session.ConnectionId, chs, ws)
			})
		}
	case model.SESSIONTYPE_CLIENT:
		// clinet only has ssh type
		if !session.HasMonitors() {
			g.Go(func() error {
				return monitSsh(ctx, session, chs)
			})
		}
	}

	key := fmt.Sprintf("%d-%s-%d", currentUser.Uid, sessionId, time.Now().Nanosecond())
	session.Monitors.Store(key, ws)
	defer func() {
		session.Monitors.Delete(key)
		if session.IsSsh() {
			if session.SessionType == model.SESSIONTYPE_CLIENT && !session.HasMonitors() {
				close(chs.AwayChan)
			}
		} else {
			close(chs.AwayChan)
		}
	}()

	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				_, p, err := ws.ReadMessage()
				if err != nil {
					logger.L.Warn("end monitor", zap.Error(err))
					return err
				}
				if !session.IsSsh() {
					chs.InChan <- p
				}
			}
		}
	})

	err = g.Wait()
}

func monitSsh(ctx *gin.Context, session *gsession.Session, chs *gsession.SessionChans) (err error) {
	req := newSshReq(ctx, model.SESSIONACTION_MONITOR)
	req.SessionId = session.SessionId
	logger.L.Debug("connect to monitor client", zap.String("sessionId", session.SessionId))
	go connectSsh(ctx, req, chs)
	if err = <-chs.ErrChan; err != nil {
		err = &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": err}}
		return
	}
	resp := <-chs.RespChan
	if resp.Code != 0 {
		err = &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": resp.Message}}
		return
	}
	tk := time.NewTicker(time.Millisecond * 100)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case closeBy := <-chs.CloseChan:
				writeToMonitors(session.Monitors, []byte("\r\n \033[31m closed by admin"))
				logger.L.Warn("close by admin", zap.String("username", closeBy))
				return nil
			case err := <-chs.ErrChan:
				logger.L.Error("ssh connection failed", zap.Error(err))
				return err
			case out := <-chs.OutChan:
				chs.Buf.Write(out)
			case <-tk.C:
				sendSshMsg(nil, session, chs)
			}
		}
	})

	if err = g.Wait(); err != nil {
		logger.L.Warn("monit ssh stopped", zap.Error(err))
	}

	return
}

func monitGuacd(ctx *gin.Context, connectionId string, chs *gsession.SessionChans, ws *websocket.Conn) (err error) {
	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	defer func() {
		chs.ErrChan <- err
	}()

	t, err := guacd.NewTunnel(connectionId, w, h, dpi, ":", nil, nil, nil)
	if err != nil {
		logger.L.Error("guacd tunnel failed", zap.Error(err))
		return
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if err != nil {
					logger.L.Debug("read instruction failed", zap.Error(err))
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
			case closeBy := <-chs.CloseChan:
				err := fmt.Errorf("colse by admin %s", closeBy)
				ws.WriteMessage(websocket.TextMessage, guacd.NewInstruction("disconnect", err.Error()).Bytes())
				logger.L.Warn(err.Error())
				return err
			case err := <-chs.ErrChan:
				logger.L.Error("disconnected", zap.Error(err))
				return err
			case out := <-chs.OutChan:
				ws.WriteMessage(websocket.TextMessage, out)
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})
	if err = g.Wait(); err != nil {
		logger.L.Warn("monit guacd stopped", zap.Error(err))
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

	logger.L.Info("closing...", zap.String("sessionId", session.SessionId), zap.Int("type", session.SessionType))
	defer offlineSession(ctx, session.SessionId, currentUser.GetUserName())
	if session.IsSsh() {
		chs := makeChans()
		req := newSshReq(ctx, model.SESSIONACTION_CLOSE)
		req.SessionId = session.SessionId
		go connectSsh(ctx, req, chs)
		if err = <-chs.ErrChan; err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": err}})
			return
		}
		resp := <-chs.RespChan
		if resp.Code != 0 {
			ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": resp.Message}})
			return
		}
	} else {
		session.Status = model.SESSIONSTATUS_OFFLINE
		session.ClosedAt = lo.ToPtr(time.Now())
		gsession.HandleUpsertSession(ctx, session)
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

func offlineSession(ctx *gin.Context, sessionId string, closer string) {
	logger.L.Debug("offline", zap.String("session_id", sessionId), zap.String("closer", closer))
	defer gsession.GetOnlineSession().Delete(sessionId)
	v, ok := gsession.GetOnlineSession().Load(sessionId)
	if ok {
		if session, ok := v.(*gsession.Session); ok {
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
					localizer := i18n.NewLocalizer(conf.Bundle, lang, accept)
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
	}
}

func checkTime(data *model.AccessAuth) bool {
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

func checkAuthorization(user *acl.Session, asset *model.Asset, accountId int) bool {
	return acl.IsAdmin(user) || lo.Contains(asset.Authorization[accountId], user.GetRid())
}

func loadOnlineSessionById(sessionId string, isMonit bool) (session *gsession.Session, err error) {
	v, ok := gsession.GetOnlineSession().Load(sessionId)
	if !ok {
		err = &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}
	session, ok = v.(*gsession.Session)
	if !ok {
		err = &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "invalid type"}}
		return
	}
	if !isMonit && session.Connected.Load() {
		err = &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}

	return
}

func handleError(ctx *gin.Context, session *gsession.Session, err error, ws *websocket.Conn) {
	defer func() {
		close(session.Chans.AwayChan)
	}()

	if err == nil {
		return
	}
	logger.L.Debug("", zap.String("session_id", session.SessionId), zap.Error(err))
	ae, ok := err.(*ApiError)
	if !ok {
		return
	}
	if session.IsSsh() {
		ws.WriteMessage(websocket.TextMessage, []byte(ae.MessageWithCtx(ctx)))
	} else {
		ws.WriteMessage(websocket.TextMessage, guacd.NewInstruction("error", (ae).MessageBase64(ctx), cast.ToString(ErrAdminClose)).Bytes())
	}
	// ctx.AbortWithError(http.StatusBadRequest, err)
}

func isCtxDone(ctx context.Context) bool {
	select {
	case _, ok := <-ctx.Done():
		return !ok
	default:
		return false
	}
}

func idleTime() (d time.Duration) {
	d = time.Hour * 2
	cfg := &model.Config{}
	if err := mysql.DB.Where(cfg).First(cfg).Error; err != nil {
		return
	}
	d = time.Second * time.Duration(cfg.Timeout)
	return
}
