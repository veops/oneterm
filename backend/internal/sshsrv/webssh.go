package sshsrv

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/connector/protocols"
	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

// HandleWebSSH handles WebSocket connections to SSH server interface
func HandleWebSSH(ctx *gin.Context) {
	// Mark this as a WebSocket connection to prevent response writing
	ctx.Set("websocket_connection", true)
	ctx.Set("sessionType", model.SESSIONTYPE_WEB)

	// Use the same upgrader as normal SSH connections
	ws, err := protocols.Upgrader.Upgrade(ctx.Writer, ctx.Request, http.Header{
		"sec-websocket-protocol": {ctx.GetHeader("sec-websocket-protocol")},
	})
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer ws.Close()
	
	// Abort the gin context to prevent any middleware from writing to response
	ctx.Abort()

	// Get current user session for authentication
	currentUser, err := acl.GetSessionFromCtx(ctx)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("Authentication required"))
		return
	}

	// Log connection
	logger.L().Info("WebSSH connection established",
		zap.String("user", currentUser.GetUserName()),
		zap.String("remote_addr", ctx.ClientIP()),
	)

	// Create a session with proper initialization
	sess := createWebSSHSession(ctx, ws, currentUser)

	// Wait for "connection" to succeed (immediately in our case)
	if err = <-sess.Chans.ErrChan; err != nil {
		logger.L().Error("failed to initialize WebSSH session", zap.Error(err))
		return
	}

	// Register session like DoConnect does
	gsession.GetOnlineSession().Store(sess.SessionId, sess)
	gsession.UpsertSession(sess)

	// Start WebSSH terminal interface
	err = runWebSSHTerminal(sess, ctx)
	if err != nil {
		// Check if this is just a normal connection close error after user exits
		if strings.Contains(err.Error(), "use of closed network connection") ||
		   strings.Contains(err.Error(), "websocket: close sent") ||
		   strings.Contains(err.Error(), "connection reset by peer") {
			logger.L().Debug("WebSSH connection closed normally", zap.Error(err))
		} else {
			logger.L().Error("WebSSH terminal failed", zap.Error(err))
		}
	}
}

// createWebSSHSession creates a session object for WebSSH
func createWebSSHSession(ctx *gin.Context, ws *websocket.Conn, currentUser *acl.Session) *gsession.Session {
	// Use the same method as DoConnect to create session
	sessionId := fmt.Sprintf("webssh-%d", time.Now().UnixNano())

	// Use gsession.NewSession for proper initialization
	sess := gsession.NewSession(ctx)
	sess.Ws = ws
	sess.Session = &model.Session{
		SessionType: model.SESSIONTYPE_WEB,
		SessionId:   sessionId,
		Uid:         currentUser.GetUid(),
		UserName:    currentUser.GetUserName(),
		AssetId:     0, // WebSSH doesn't connect to a specific asset
		AssetInfo:   "WebSSH Terminal",
		AccountId:   0, // WebSSH doesn't use a specific account
		AccountInfo: "WebSSH",
		GatewayId:   0,
		GatewayInfo: "",
		Protocol:    "webssh",
		Status:      model.SESSIONSTATUS_ONLINE,
		ClientIp:    ctx.ClientIP(),
	}

	// Initialize SshParser like DoConnect does for non-Guacd protocols
	w, h := 80, 24 // Default terminal size
	if wStr := ctx.Query("w"); wStr != "" {
		if wInt, err := strconv.Atoi(wStr); err == nil && wInt > 0 {
			w = wInt
		}
	}
	if hStr := ctx.Query("h"); hStr != "" {
		if hInt, err := strconv.Atoi(hStr); err == nil && hInt > 0 {
			h = hInt
		}
	}

	sess.SshParser = gsession.NewParser(sess.SessionId, w, h)
	sess.SshParser.Protocol = sess.Protocol

	// Initialize SSH recorder
	if recorder, err := gsession.NewAsciinema(sess.SessionId, w, h); err == nil {
		sess.SshRecoder = recorder
	}

	// For WebSSH, we don't connect to external server, so signal success immediately
	go func() {
		// Send nil to ErrChan to indicate successful "connection"
		sess.Chans.ErrChan <- nil
	}()

	return sess
}

// runWebSSHTerminal implements the WebSSH terminal interface
func runWebSSHTerminal(sess *gsession.Session, ctx *gin.Context) error {
	defer func() {
		// Clean up session like HandleTerm does
		if sess.SshRecoder != nil {
			if closeErr := sess.SshRecoder.Close(); closeErr != nil {
				logger.L().Error("Failed to close SSH recorder", zap.String("sessionId", sess.SessionId), zap.Error(closeErr))
			}
		}
		sess.SshParser.Close(sess.Prompt)
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = &time.Time{}
		*sess.ClosedAt = time.Now()
		gsession.UpsertSession(sess)
	}()

	// Use the same pattern as terminal SSH handler
	sess.G.Go(func() error {
		return protocols.Read(sess)
	})

	sess.G.Go(func() error {
		defer sess.Chans.Rin.Close()
		defer sess.Chans.Wout.Close()

		// Set up bubbletea like HandleTerm does
		currentUser, _ := acl.GetSessionFromCtx(ctx)
		fakeSSHSession := &fakeWebSSHSession{
			sess: sess,
			user: currentUser,
		}

		// Create pipes like terminal SSH
		r, w := io.Pipe()

		// Process InChan and forward to pipe (handles WebSocket protocol prefixes)
		go func() {
			defer w.Close()
			chs := sess.Chans

			for {
				select {
				case <-sess.Gctx.Done():
					return
				case in := <-chs.InChan:
					// Handle WebSocket protocol prefixes exactly like HandleTerm does
					if sess.SessionType == model.SESSIONTYPE_WEB {
						rt := in[0]
						msg := in[1:]
						switch rt {
						case '1':
							in = msg // Strip '1' prefix for input
						case '9':
							continue // Skip heartbeat messages
						case 'w':
							// Handle window resize - for WebSSH, we don't need to send to WindowChan
							// since bubbletea handles its own resize events through tea.WindowSizeMsg
							// Just log and continue to avoid blocking the input processing
							wh := strings.Split(string(msg), ",")
							if len(wh) >= 2 {
								logger.L().Debug("WebSSH window resize", 
									zap.Int("width", cast.ToInt(wh[0])),
									zap.Int("height", cast.ToInt(wh[1])))
							}
							continue
						}
					}

					// Write processed input to pipe
					if _, err := w.Write(in); err != nil {
						return
					}
				}
			}
		}()

		defer r.Close()
		defer w.Close()

		// Run bubbletea exactly like terminal SSH
		vw := initialView(ctx, fakeSSHSession, r, w, sess.Gctx)
		defer vw.RecordHisCmd()

		p := tea.NewProgram(vw, tea.WithContext(sess.Gctx), tea.WithInput(r), tea.WithOutput(fakeSSHSession))

		_, err := p.Run()
		if err != nil {
			logger.L().Error("bubbletea program error", zap.Error(err))
		}

		// When bubbletea exits (e.g., user typed "exit"), ensure proper cleanup
		logger.L().Debug("bubbletea program ended, initiating cleanup")
		
		// Give a small delay to ensure the "Goodbye!" message is sent to frontend
		time.Sleep(100 * time.Millisecond)
		
		// Signal other goroutines to stop by closing AwayChan
		// This will cause protocols.Read to return and stop gracefully
		sess.Once.Do(func() {
			logger.L().Debug("Closing AwayChan to signal other goroutines")
			close(sess.Chans.AwayChan)
		})
		
		// Close WebSocket after signaling other goroutines to stop
		if sess.Ws != nil {
			sess.Ws.Close()
			logger.L().Debug("WebSocket connection closed")
		}
		
		return err
	})

	// Wait for all goroutines to complete
	return sess.G.Wait()
}

// fakeWebSSHSession implements ssh.Session interface for compatibility with initialView
type fakeWebSSHSession struct {
	sess *gsession.Session
	user *acl.Session
}

func (f *fakeWebSSHSession) User() string                 { return f.user.GetUserName() }
func (f *fakeWebSSHSession) RemoteAddr() net.Addr         { return &fakeAddr{addr: "webssh-client"} }
func (f *fakeWebSSHSession) LocalAddr() net.Addr          { return &fakeAddr{addr: "webssh-server"} }
func (f *fakeWebSSHSession) Environ() []string            { return []string{"TERM=xterm-256color"} }
func (f *fakeWebSSHSession) Command() []string            { return []string{} }
func (f *fakeWebSSHSession) RawCommand() string           { return "" }
func (f *fakeWebSSHSession) Subsystem() string            { return "" }
func (f *fakeWebSSHSession) PublicKey() ssh.PublicKey     { return nil }
func (f *fakeWebSSHSession) Permissions() ssh.Permissions { return ssh.Permissions{} }
func (f *fakeWebSSHSession) Exit(code int) error          { return nil }
func (f *fakeWebSSHSession) Read(p []byte) (n int, err error) {
	// This is for stderr reads, not typically used
	return 0, io.EOF
}
func (f *fakeWebSSHSession) Write(p []byte) (n int, err error) {
	// Write to OutBuf like protocols.WriteErrMsg does
	n, err = f.sess.Chans.OutBuf.Write(p)
	if err == nil {
		// Trigger immediate write to WebSocket
		protocols.Write(f.sess)
	}
	return n, err
}
func (f *fakeWebSSHSession) CloseWrite() error     { return nil }
func (f *fakeWebSSHSession) Stderr() io.ReadWriter { return f }
func (f *fakeWebSSHSession) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return false, nil
}
func (f *fakeWebSSHSession) Signals(c chan<- ssh.Signal) {}
func (f *fakeWebSSHSession) Break(c chan<- bool)         {}
func (f *fakeWebSSHSession) Close() error                { return nil }

func (f *fakeWebSSHSession) Context() ssh.Context {
	return &fakeSSHContext{user: f.user.GetUserName()}
}

func (f *fakeWebSSHSession) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	ch := make(chan ssh.Window)
	close(ch)
	return ssh.Pty{Term: "xterm-256color", Window: ssh.Window{Width: 80, Height: 24}}, ch, true
}

// fakeAddr implements net.Addr
type fakeAddr struct {
	addr string
}

func (f *fakeAddr) Network() string { return "webssh" }
func (f *fakeAddr) String() string  { return f.addr }

// fakeSSHContext implements ssh.Context interface
type fakeSSHContext struct {
	user string
}

func (f *fakeSSHContext) Deadline() (deadline time.Time, ok bool) { return time.Time{}, false }
func (f *fakeSSHContext) Done() <-chan struct{}                   { return nil }
func (f *fakeSSHContext) Err() error                              { return nil }
func (f *fakeSSHContext) Value(key any) any                       { return nil }
func (f *fakeSSHContext) User() string                            { return f.user }
func (f *fakeSSHContext) SessionID() string                       { return "webssh-session" }
func (f *fakeSSHContext) ClientVersion() string                   { return "WebSSH-1.0" }
func (f *fakeSSHContext) ServerVersion() string                   { return "OneTerm-WebSSH-1.0" }
func (f *fakeSSHContext) RemoteAddr() net.Addr                    { return &fakeAddr{addr: "webssh-client"} }
func (f *fakeSSHContext) LocalAddr() net.Addr                     { return &fakeAddr{addr: "webssh-server"} }
func (f *fakeSSHContext) Permissions() *ssh.Permissions           { return &ssh.Permissions{} }
func (f *fakeSSHContext) SetValue(key, value any)                 {}
func (f *fakeSSHContext) Lock()                                   {}
func (f *fakeSSHContext) Unlock()                                 {}
