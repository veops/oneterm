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
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/veops/oneterm/pkg/conf"
	myi18n "github.com/veops/oneterm/pkg/i18n"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
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
//	@Success	200	{object}	HttpResponse
//	@Param		session_id	path		int	true	"session id"
//	@Router		/connect/:session_id [get]
func (c *Controller) Connecting(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	defer func() {
		if err == nil {
			return
		}
		logger.L.Debug("connecting failed", zap.String("session_id", sessionId), zap.Error(err))
		ae, ok := err.(*ApiError)
		if !ok {
			return
		}
		lang := ctx.PostForm("lang")
		accept := ctx.GetHeader("Accept-Language")
		localizer := i18n.NewLocalizer(conf.Bundle, lang, accept)
		ws.WriteMessage(websocket.TextMessage, []byte(ae.Message(localizer)))
	}()

	v, ok := onlineSession.Load(sessionId)
	if !ok {
		err = &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}
	session, ok := v.(*model.Session)
	if !ok {
		err = &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "invalid type"}}
		return
	}
	if session.Connected.Load() {
		err = &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}}
		return
	}
	session.Connected.CompareAndSwap(false, true)

	chs := session.Chans
	chs.WindowChan <- fmt.Sprintf("%s,%s", ctx.Query("w"), ctx.Query("h"))
	defer func() {
		close(chs.AwayChan)
	}()
	readWsErrChan := make(chan error)
	go func() {
		readWsErrChan <- readWsMsg(ctx, ws, chs)
	}()
	tk, tk1s := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second)
	defer sendMsg(ws, session, chs)
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-readWsErrChan:
			logger.L.Error("websocket read failed", zap.Error(err))
			return
		case closeBy := <-chs.CloseChan:
			out := []byte("\r\n \033[31m closed by admin")
			ws.WriteMessage(websocket.TextMessage, out)
			writeToMonitors(session.Monitors, out)
			logger.L.Warn("close by admin", zap.String("username", closeBy))
			return
		case err := <-chs.ErrChan:
			logger.L.Error("ssh connection failed", zap.Error(err))
			return
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
			sendMsg(ws, session, chs)
		case <-tk1s.C:
			ws.WriteMessage(websocket.TextMessage, nil)
			writeToMonitors(session.Monitors, nil)
		}
	}
}

func sendMsg(ws *websocket.Conn, session *model.Session, chs *model.SessionChans) {
	out := chs.Buf.Bytes()
	if len(out) <= 0 {
		return
	}
	if ws != nil {
		ws.WriteMessage(websocket.TextMessage, out)
	}
	writeToMonitors(session.Monitors, out)
	chs.Buf.Reset()
}

// Connect godoc
//
//	@Tags		connect
//	@Success	200	{object}	HttpResponse
//	@Param		w	query		int	false	"width"
//	@Param		h	query		int	false	"height"
//	@Success	200			{object}	HttpResponse{data=model.Session}
//	@Router		/connect/:asset_id/:account_id/:protocol [post]
func (c *Controller) Connect(ctx *gin.Context) {
	chs := makeChans()

	resp := &model.SshResp{}
	go doSsh(ctx, cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), newSshReq(ctx, model.SESSIONACTION_NEW), chs)
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
	v, ok := onlineSession.Load(resp.SessionId)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "cannot find in sync map"}})
		return
	}
	session, ok := v.(*model.Session)
	if !ok {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrLoadSession, Data: map[string]any{"err": "invalid type"}})
		return
	}
	session.Chans = chs

	ctx.JSON(http.StatusOK, NewHttpResponseWithData(session))
}

func readWsMsg(ctx context.Context, ws *websocket.Conn, chs *model.SessionChans) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("ctx done")
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
			}
		}
	}
}

func doSsh(ctx *gin.Context, w, h int, req *model.SshReq, chs *model.SessionChans) {
	var err error
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
	resp := &model.SshResp{}
	if err = json.Unmarshal([]byte(line)[0:len(line)-1], resp); err != nil {
		logger.L.Error("ssh resp", zap.Error(err), zap.String("resp content", string(line)))
		return
	}

	chs.ErrChan <- nil
	chs.RespChan <- resp

	waitChan := make(chan error)
	go func() {
		waitChan <- sess.Wait()
	}()
	go func() {
		for {
			rn, size, err := buf.ReadRune()
			if err != nil {
				logger.L.Debug("buf ReadRune failed", zap.Error(err))
				return
			}
			if size <= 0 || rn == utf8.RuneError {
				continue
			}
			p := make([]byte, utf8.RuneLen(rn))
			utf8.EncodeRune(p, rn)
			chs.OutChan <- p
		}
	}()
	defer sess.Close()

	for {
		select {
		case err = <-waitChan:
			return
		case <-chs.AwayChan:
			return
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
}

func makeChans() *model.SessionChans {
	rin, win := io.Pipe()
	return &model.SessionChans{
		Rin:        rin,
		Win:        win,
		ErrChan:    make(chan error),
		RespChan:   make(chan *model.SshResp),
		InChan:     make(chan []byte),
		OutChan:    make(chan []byte),
		Buf:        &bytes.Buffer{},
		WindowChan: make(chan string),
		AwayChan:   make(chan struct{}),
		CloseChan:  make(chan string),
	}
}

func newSshReq(ctx *gin.Context, action int) *model.SshReq {
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	return &model.SshReq{
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
	key := fmt.Sprintf("%d-%s-%d", currentUser.Uid, sessionId, time.Now().Nanosecond())
	ws, err := Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()

	defer func() {
		if err == nil {
			return
		}
		logger.L.Debug("monitor failed", zap.String("session_id", sessionId), zap.Error(err))
		ae, ok := err.(*ApiError)
		if !ok {
			return
		}
		lang := ctx.PostForm("lang")
		accept := ctx.GetHeader("Accept-Language")
		localizer := i18n.NewLocalizer(conf.Bundle, lang, accept)
		ws.WriteMessage(websocket.TextMessage, []byte(ae.Message(localizer)))
		ctx.AbortWithError(http.StatusBadRequest, err)
	}()

	if !acl.IsAdmin(currentUser) {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrNoPerm, Data: map[string]any{"perm": "monitor session"}})
		return
	}

	session := &model.Session{}
	err = mysql.DB.
		Where("session_id = ?", sessionId).
		Where("status = ?", model.SESSIONSTATUS_ONLINE).
		First(session).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			onlineSession.Delete(sessionId)
		}
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}})
		return
	}

	v, ok := onlineSession.Load(sessionId)
	if !ok {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}})
		return
	}
	session, ok = v.(*model.Session)
	if !ok {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidSessionId, Data: map[string]any{"sessionId": sessionId}})
		return
	}
	switch session.SessionType {
	case model.SESSIONTYPE_WEB:
	case model.SESSIONTYPE_CLIENT:
		cur := false
		session.Monitors.Range(func(key, value any) bool {
			cur = true
			return !cur
		})
		if !cur {
			req := newSshReq(ctx, model.SESSIONACTION_MONITOR)
			req.SessionId = sessionId
			chs := makeChans()
			logger.L.Debug("connect to monitor client", zap.String("sessionId", sessionId))
			go doSsh(ctx, cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), req, chs)
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
			defer sendMsg(nil, session, chs)
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case closeBy := <-chs.CloseChan:
						writeToMonitors(session.Monitors, []byte("\r\n \033[31m closed by admin"))
						logger.L.Warn("close by admin", zap.String("username", closeBy))
						return
					case err := <-chs.ErrChan:
						logger.L.Error("ssh connection failed", zap.Error(err))
						return
					case out := <-chs.OutChan:
						chs.Buf.Write(out)
					case <-tk.C:
						sendMsg(nil, session, chs)
					}
				}
			}()
		}
	}

	session.Monitors.Store(key, ws)
	defer func() {
		session.Monitors.Delete(key)
	}()
	for {
		_, _, err = ws.ReadMessage()
		if err != nil {
			logger.L.Warn("end monitor", zap.Error(err))
			return
		}
	}
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

	session := &model.Session{}
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
	defer doOfflineOnlineSession(ctx, session.SessionId, currentUser.GetUserName())
	chs := makeChans()
	req := newSshReq(ctx, model.SESSIONACTION_CLOSE)
	req.SessionId = session.SessionId
	go doSsh(ctx, cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), req, chs)
	if err = <-chs.ErrChan; err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrConnectServer, Data: map[string]any{"err": err}})
		return
	}
	resp := <-chs.RespChan
	if resp.Code != 0 {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrBadRequest, Data: map[string]any{"err": resp.Message}})
		return
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

func doOfflineOnlineSession(ctx *gin.Context, sessionId string, closer string) {
	logger.L.Debug("offline", zap.String("session_id", sessionId), zap.String("closer", closer))
	defer onlineSession.Delete(sessionId)
	v, ok := onlineSession.Load(sessionId)
	if ok {
		if session, ok := v.(*model.Session); ok {
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
