package sshsrv

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	myConnector "github.com/veops/oneterm/internal/connector"
	"github.com/veops/oneterm/internal/connector/protocols"
	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

func HandleWebSSH(ctx *gin.Context) {
	user, err := acl.GetSessionFromCtx(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ctx.Set("websocket_connection", true)
	ctx.Set("sessionType", model.SESSIONTYPE_WEB)
	ws, err := protocols.Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")}})
	if err != nil {
		return
	}
	defer ws.Close()
	ctx.Abort()
	sess := createWebSSHSession(ctx, ws, user)
	defer myConnector.CloseTerminalSession(sess)
	if ctx.Query("flow") == "true" && sess.BinaryOutput {
		if err := sess.StartOutputFlow(); err != nil {
			return
		}
	}
	if sess.SshRecoder == nil {
		return
	}
	if err := gsession.UpsertSession(sess); err != nil {
		logger.L().Error("create WebSSH session failed", zap.Error(err))
		return
	}
	gsession.GetOnlineSession().Store(sess.SessionId, sess)
	if err := runWebSSHTerminal(sess, ctx); err != nil && !errors.Is(err, io.EOF) {
		logger.L().Debug("WebSSH terminal stopped", zap.Error(err))
	}
}

func createWebSSHSession(ctx *gin.Context, ws *websocket.Conn, user *acl.Session) *gsession.Session {
	sess := gsession.NewSession(ctx.Request.Context())
	sess.Ws = ws
	sess.BinaryOutput = ctx.Query("binary") == "true"
	sess.Session = &model.Session{SessionType: model.SESSIONTYPE_WEB, SessionId: "webssh-" + uuid.NewString(), Uid: user.GetUid(),
		UserName: user.GetUserName(), AssetInfo: "WebSSH Terminal", AccountInfo: "WebSSH", Protocol: "webssh", Status: model.SESSIONSTATUS_ONLINE, ClientIp: ctx.ClientIP()}
	w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
	if w <= 0 || w > 1000 {
		w = 80
	}
	if h <= 0 || h > 500 {
		h = 24
	}
	sess.SshParser = gsession.NewParser(sess.SessionId, w, h)
	sess.SshParser.Protocol = sess.Protocol
	sess.SshRecoder, _ = gsession.NewAsciinema(sess.SessionId, w, h)
	return sess
}

func runWebSSHTerminal(sess *gsession.Session, ctx *gin.Context) error {
	pty := ssh.Pty{Term: "xterm-256color", Window: ssh.Window{Width: cast.ToInt(ctx.Query("w")), Height: cast.ToInt(ctx.Query("h"))}}
	remote, _ := net.ResolveTCPAddr("tcp", ctx.Request.RemoteAddr)
	terminal := newTerminal(pty, remote, "COLORTERM=truecolor")
	input := &webSSHInput{sess: sess, terminal: terminal}
	output := &webSSHOutput{sess: sess}
	ctx = ctx.Copy()
	ctx.Request = ctx.Request.Clone(sess.Gctx)
	sess.G.Go(func() error {
		defer sess.Stop()
		return runTerminal(ctx, terminal, input, output, nil)
	})
	sess.G.Go(func() error {
		return myConnector.WatchTerminalSession(sess, ctx)
	})
	return sess.G.Wait()
}

type webSSHInput struct {
	sess     *gsession.Session
	terminal *terminalState
	pending  []byte
}

func (r *webSSHInput) Close() error { return r.sess.Ws.Close() }

func (r *webSSHInput) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		r.sess.Ws.SetReadLimit(1024 * 1024)
		r.sess.Ws.SetReadDeadline(time.Now().Add(max(2*time.Minute, gsession.IdleTimeout())))
		_, data, err := r.sess.Ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		if handled, err := r.sess.HandleOutputAck(data); handled {
			if err != nil {
				return 0, err
			}
			continue
		}
		input, window := protocols.TerminalMessage(data)
		if window.Width > 0 {
			r.sess.SetIdle()
			r.terminal.Resize(window)
			continue
		}
		if len(input) == 0 {
			continue
		}
		r.sess.SetIdle()
		r.pending = input
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

type webSSHOutput struct{ sess *gsession.Session }

func (w *webSSHOutput) Write(p []byte) (int, error) {
	return w.WriteContext(w.sess.Gctx, p)
}

func (w *webSSHOutput) WriteContext(ctx context.Context, p []byte) (int, error) {
	if err := w.sess.WriteTerminalContext(ctx, p); err != nil {
		return 0, err
	}
	if w.sess.SshRecoder != nil {
		w.sess.SshRecoder.Write(p)
	}
	protocols.WriteToMonitors(w.sess.Monitors, p)
	return len(p), nil
}
