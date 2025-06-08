package session

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"
	"time"
	"unicode/utf8"

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

func init() {
	// After system restart, set all online sessions to offline
	sessions := make([]*Session, 0)
	if err := dbpkg.DB.
		Model(sessions).
		Where("status = ?", model.SESSIONSTATUS_ONLINE).
		Find(&sessions).
		Error; err != nil {
		logger.L().Fatal("get sessions failed", zap.Error(err))
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
	rn, size, err := rw.Reader.ReadRune()
	if err != nil {
		return
	}
	if size <= 0 || rn == utf8.RuneError {
		return
	}
	p = make([]byte, utf8.RuneLen(rn))
	utf8.EncodeRune(p, rn)
	return
}

func (rw *CliRW) Write(p []byte) (n int, err error) {
	return rw.Writer.Write(p)
}

type SessionChans struct {
	Rin        io.ReadCloser
	Win        io.WriteCloser
	Rout       io.ReadCloser
	Wout       io.WriteCloser
	ErrChan    chan error
	InChan     chan []byte
	OutChan    chan []byte
	OutBuf     *bytes.Buffer
	WindowChan chan ssh.Window
	AwayChan   chan struct{}
	CloseChan  chan string
}

func NewSessionChans() *SessionChans {
	rin, win := io.Pipe()
	rout, wout := io.Pipe()
	return &SessionChans{
		Rin:        rin,
		Win:        win,
		Rout:       rout,
		Wout:       wout,
		ErrChan:    make(chan error),
		InChan:     make(chan []byte, 8),
		OutChan:    make(chan []byte, 8),
		OutBuf:     &bytes.Buffer{},
		WindowChan: make(chan ssh.Window),
		AwayChan:   make(chan struct{}),
		CloseChan:  make(chan string),
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
}

func (m *Session) HasMonitors() (has bool) {
	m.Monitors.Range(func(key, value any) bool {
		has = true
		return false
	})
	return
}

func (m *Session) SetIdle() {
	d := time.Hour
	cfg := model.GlobalConfig.Load()
	if cfg != nil && cfg.Timeout > 0 {
		d = time.Second * time.Duration(cfg.Timeout)
	}
	if m.IdleTk == nil {
		m.IdleTk = time.NewTicker(d)
	} else {
		m.IdleTk.Reset(d)
	}

}

func NewSession(ctx context.Context) *Session {
	s := &Session{}
	s.G, s.Gctx = errgroup.WithContext(ctx)
	s.Chans = NewSessionChans()
	s.Monitors = &sync.Map{}
	s.SetIdle()
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
