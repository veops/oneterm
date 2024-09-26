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
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm/clause"

	"github.com/veops/oneterm/api/guacd"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

var (
	onlineSession = &sync.Map{}
)

func init() {
	sessions := make([]*Session, 0)
	if err := mysql.DB.
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

func (rw *CliRW) Read() []byte {
	rn, size, err := rw.Reader.ReadRune()
	if err != nil {
		return nil
	}
	if size <= 0 || rn == utf8.RuneError {
		return nil
	}
	p := make([]byte, utf8.RuneLen(rn))
	utf8.EncodeRune(p, rn)
	return p
}

func (rw *CliRW) Write(p []byte) {
	rw.Writer.Write(p)
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
	return mysql.DB.
		Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"status", "closed_at"}),
		}).
		Create(data).
		Error
}
