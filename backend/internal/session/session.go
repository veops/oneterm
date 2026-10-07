package session

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm/clause"

	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	onlineSession = &sync.Map{}
)

// InitSessionCleanup initializes session cleanup after database is ready
func InitSessionCleanup() {
	// After system restart, set all online sessions to offline
	sessions := make([]*Session, 0)
	if err := dbpkg.DB.
		Model(sessions).
		Where("status = ?", model.SESSIONSTATUS_ONLINE).
		Find(&sessions).
		Error; err != nil {
		logger.L().Error("get sessions failed", zap.Error(err))
		return
	}
	now := time.Now()
	for _, s := range sessions {
		s.Status = model.SESSIONSTATUS_OFFLINE
		s.ClosedAt = &now
		UpsertSession(s)
	}
}

func GetOnlineSession() *sync.Map {
	return onlineSession
}

func GetOnlineSessionById(id string) (sess *Session) {
	v, ok := GetOnlineSession().Load(id)
	if !ok {
		return nil
	}
	return v.(*Session)
}

type CliRW struct {
	Reader *bufio.Reader
	Writer io.Writer
}

func (rw *CliRW) Read() (p []byte, err error) {
	p = make([]byte, 32768)
	n, err := rw.Reader.Read(p)
	return p[:n], err
}

func (rw *CliRW) Write(p []byte) (n int, err error) {
	return rw.Writer.Write(p)
}

func (rw *CliRW) WriteContext(ctx context.Context, p []byte) (int, error) {
	if writer, ok := rw.Writer.(interface {
		WriteContext(context.Context, []byte) (int, error)
	}); ok {
		return writer.WriteContext(ctx, p)
	}
	return rw.Write(p)
}

type SessionChans struct {
	Rin        io.ReadCloser
	Win        io.WriteCloser
	Rout       io.ReadCloser
	Wout       io.WriteCloser
	ErrChan    chan error
	InChan     chan []byte
	OutChan    chan []byte
	OutBuf     *Buffer
	WindowChan chan ssh.Window
	AwayChan   chan struct{}
	CloseChan  chan string
	windowMu   sync.Mutex
	closeOnce  sync.Once
}

type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *Buffer) WriteString(s string) (int, error) { return b.Write([]byte(s)) }

func (b *Buffer) Len() int { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.Len() }

func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *Buffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func (b *Buffer) Drain() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	p := b.buf.Bytes()
	b.buf = bytes.Buffer{}
	return p
}

func NewSessionChans() *SessionChans {
	rin, win := io.Pipe()
	rout, wout := io.Pipe()
	return &SessionChans{
		Rin:        rin,
		Win:        win,
		Rout:       rout,
		Wout:       wout,
		ErrChan:    make(chan error, 1),
		InChan:     make(chan []byte, 8),
		OutChan:    make(chan []byte, 8),
		OutBuf:     &Buffer{},
		WindowChan: make(chan ssh.Window, 1),
		AwayChan:   make(chan struct{}),
		CloseChan:  make(chan string),
	}
}

func (c *SessionChans) Resize(window ssh.Window) {
	if window.Width <= 0 || window.Height <= 0 || window.Width > 1000 || window.Height > 500 {
		return
	}
	c.windowMu.Lock()
	defer c.windowMu.Unlock()
	select {
	case c.WindowChan <- window:
	default:
		select {
		case <-c.WindowChan:
		default:
		}
		c.WindowChan <- window
	}
}

func (c *SessionChans) CloseAway() { c.closeOnce.Do(func() { close(c.AwayChan) }) }

func (c *SessionChans) SendOutput(ctx context.Context, data []byte) bool {
	select {
	case <-ctx.Done():
		return false
	case <-c.AwayChan:
		return false
	case c.OutChan <- data:
		return true
	}
}

type Session struct {
	*model.Session
	G            *errgroup.Group `json:"-" gorm:"-"`
	Gctx         context.Context `json:"-" gorm:"-"`
	Ws           *websocket.Conn `json:"-" gorm:"-"`
	CliRw        *CliRW          `json:"-" gorm:"-"`
	Monitors     *sync.Map       `json:"-" gorm:"-"`
	Chans        *SessionChans   `json:"-" gorm:"-"`
	ConnectionId string          `json:"-" gorm:"-"`
	GuacdTunnel  *guacd.Tunnel   `json:"-" gorm:"-"`
	IdleTk       *time.Ticker    `json:"-" gorm:"-"`
	SshRecoder   *Asciinema      `json:"-" gorm:"-"`
	SshParser    *Parser         `json:"-" gorm:"-"`
	ShareEnd     time.Time       `json:"-" gorm:"-"`
	Once         sync.Once       `json:"-" gorm:"-"`
	Prompt       string          `json:"-" gorm:"-"`

	// SSH connection reuse for file transfers
	SSHClient *gossh.Client `json:"-" gorm:"-"`
	sshMutex  sync.RWMutex  `json:"-" gorm:"-"`

	// Web session support
	WebSession         interface{}                `json:"-" gorm:"-"`
	Permissions        *model.AuthPermissions     `json:"-" gorm:"-"`
	PAMAuthorization   *model.PAMConnectionPermit `json:"-" gorm:"-"`
	pamTransportMu     sync.Mutex
	pamTransportClose  func()
	pamTransportClosed bool
	cancel             context.CancelFunc
	wsMutex            sync.Mutex
	idleMutex          sync.Mutex
	idleStopped        bool
	BinaryOutput       bool        `json:"-" gorm:"-"`
	OutputFlow         *OutputFlow `json:"-" gorm:"-"`
	textPending        []byte
}

func (s *Session) Stop() {
	s.Once.Do(func() { close(s.Chans.AwayChan) })
	if s.cancel != nil {
		s.cancel()
	}
	s.CloseTransport()
	s.ClearSSHClient()
	s.StopIdle()
	if s.Ws != nil {
		s.Ws.Close()
	}
	s.Chans.Rin.Close()
	s.Chans.Win.Close()
	s.Chans.Rout.Close()
	s.Chans.Wout.Close()
}

func (s *Session) WriteWebsocket(kind int, p []byte) error {
	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()
	return s.writeWebsocket(kind, p)
}

func (s *Session) writeWebsocket(kind int, p []byte) error {
	if kind == websocket.TextMessage && len(p) > 0 {
		p, s.textPending = terminalText(s.textPending, p)
		if len(p) == 0 {
			return nil
		}
	}
	if err := s.Ws.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return s.Ws.WriteMessage(kind, p)
}

func (s *Session) WriteTerminal(p []byte) error {
	return s.WriteTerminalContext(s.Gctx, p)
}

func (s *Session) WriteTerminalContext(ctx context.Context, p []byte) error {
	if s.OutputFlow != nil {
		s.wsMutex.Lock()
		defer s.wsMutex.Unlock()
		for len(p) > 0 {
			size := min(len(p), 32768)
			if err := s.OutputFlow.reserve(ctx, s.Gctx, size); err != nil {
				return err
			}
			if err := s.writeWebsocket(websocket.BinaryMessage, p[:size]); err != nil {
				return err
			}
			p = p[size:]
		}
		return nil
	}
	kind := websocket.TextMessage
	if s.BinaryOutput {
		kind = websocket.BinaryMessage
	}
	return s.WriteWebsocket(kind, p)
}

func (s *Session) StopIdle() {
	s.idleMutex.Lock()
	defer s.idleMutex.Unlock()
	s.idleStopped = true
	if s.IdleTk != nil {
		s.IdleTk.Stop()
	}
}

func (s *Session) SetPAMTransportClose(closeTransport func()) {
	if s.PAMAuthorization == nil {
		return
	}
	s.SetTransportClose(closeTransport)
}

func (s *Session) SetTransportClose(closeTransport func()) {
	s.pamTransportMu.Lock()
	alreadyClosed := s.pamTransportClosed
	if !alreadyClosed {
		s.pamTransportClose = closeTransport
	}
	s.pamTransportMu.Unlock()
	if alreadyClosed {
		closeTransport()
	}
}

func (s *Session) ClosePAMTransport() {
	s.CloseTransport()
}

func (s *Session) CloseTransport() {
	s.pamTransportMu.Lock()
	closeTransport := s.pamTransportClose
	s.pamTransportClosed, s.pamTransportClose = true, nil
	s.pamTransportMu.Unlock()
	if closeTransport != nil {
		closeTransport()
	}
}

func (m *Session) HasMonitors() (has bool) {
	m.Monitors.Range(func(key, value any) bool {
		has = true
		return false
	})
	return
}

func IdleTimeout() time.Duration {
	d := time.Hour
	cfg := model.GlobalConfig.Load()
	if cfg != nil && cfg.Timeout > 0 {
		d = time.Second * time.Duration(cfg.Timeout)
	}
	return d
}

func (m *Session) SetIdle() {
	m.idleMutex.Lock()
	defer m.idleMutex.Unlock()
	if m.idleStopped {
		return
	}
	d := IdleTimeout()
	if m.IdleTk == nil {
		m.IdleTk = time.NewTicker(d)
	} else {
		m.IdleTk.Reset(d)
	}

}

func NewSession(ctx context.Context) *Session {
	s := &Session{}
	ctx, s.cancel = context.WithCancel(ctx)
	s.G, s.Gctx = errgroup.WithContext(ctx)
	s.Chans = NewSessionChans()
	s.Monitors = &sync.Map{}
	s.SetIdle()
	context.AfterFunc(s.Gctx, s.Stop)
	return s
}

func UpsertSession(data *Session) (err error) {
	return dbpkg.DB.
		Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"status", "closed_at"}),
		}).
		Create(data).
		Error
}

// SetSSHClient stores SSH client for connection reuse
func (s *Session) SetSSHClient(client *gossh.Client) {
	s.sshMutex.Lock()
	defer s.sshMutex.Unlock()
	s.SSHClient = client
	logger.L().Debug("SSH client stored for session", zap.String("sessionId", s.SessionId))
}

// GetSSHClient gets stored SSH client for connection reuse
func (s *Session) GetSSHClient() *gossh.Client {
	s.sshMutex.RLock()
	defer s.sshMutex.RUnlock()
	return s.SSHClient
}

// ClearSSHClient clears stored SSH client
func (s *Session) ClearSSHClient() {
	s.sshMutex.Lock()
	defer s.sshMutex.Unlock()
	if s.SSHClient != nil {
		s.SSHClient.Close()
		s.SSHClient = nil
		logger.L().Debug("SSH client cleared for session", zap.String("sessionId", s.SessionId))
	}
}

// HasSSHClient checks if session has an active SSH client
func (s *Session) HasSSHClient() bool {
	s.sshMutex.RLock()
	defer s.sshMutex.RUnlock()
	return s.SSHClient != nil
}

// SetWebSession stores a Web session object
func (s *Session) SetWebSession(webSession interface{}) {
	s.WebSession = webSession
}

// GetWebSession returns the stored Web session object
func (s *Session) GetWebSession() interface{} {
	return s.WebSession
}

// SetPermissions stores the user permissions for this session
func (s *Session) SetPermissions(permissions *model.AuthPermissions) {
	s.Permissions = permissions
}

// GetPermissions returns the stored permissions
func (s *Session) GetPermissions() *model.AuthPermissions {
	return s.Permissions
}
