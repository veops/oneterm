package session

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/guacd"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/server/storage/db/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

var (
	onlineSession = &sync.Map{}
)

func GetOnlineSession() *sync.Map {
	return onlineSession
}

type SshReq struct {
	Uid            int    `json:"uid"`
	UserName       string `json:"username"`
	Cookie         string `json:"cookie"`
	AcceptLanguage string `json:"accept_language"`
	ClientIp       string `json:"client_ip"`
	AssetId        int    `json:"asset_id"`
	AccountId      int    `json:"account_id"`
	Protocol       string `json:"protocol"`
	Action         int    `json:"action"`
	SessionId      string `json:"session_id"`
}

type ServerResp struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	SessionId string `json:"session_id"`
	Uid       int    `json:"uid"`
	UserName  string `json:"username"`
}

type SessionChans struct {
	Rin        io.Reader
	Win        io.Writer
	ErrChan    chan error
	RespChan   chan *ServerResp
	InChan     chan []byte
	OutChan    chan []byte
	Buf        *bytes.Buffer
	WindowChan chan string
	AwayChan   chan struct{}
	CloseChan  chan string
}

type Session struct {
	*model.Session
	Monitors     *sync.Map     `json:"-" gorm:"-"`
	Chans        *SessionChans `json:"-" gorm:"-"`
	Connected    atomic.Bool   `json:"-" gorm:"-"`
	ConnectionId string        `json:"-" gorm:"-"`
	GuacdTunnel  *guacd.Tunnel `json:"-" gorm:"-"`
	IdleTimout   time.Duration `json:"-" gorm:"-"`
	IdleTk       *time.Ticker  `json:"-" gorm:"-"`
}

func (m *Session) HasMonitors() (has bool) {
	m.Monitors.Range(func(key, value any) bool {
		has = true
		return false
	})
	return
}

func Init() (err error) {
	sessions := make([]*Session, 0)
	if err = mysql.DB.
		Model(sessions).
		Where("status = ?", model.SESSIONSTATUS_ONLINE).
		Find(&sessions).
		Error; err != nil {
		logger.L.Warn("get sessions failed", zap.Error(err))
		return
	}
	ctx := &gin.Context{}
	now := time.Now()
	for _, s := range sessions {
		if s.SessionType == model.SESSIONTYPE_WEB {
			s.Status = model.SESSIONSTATUS_OFFLINE
			s.ClosedAt = &now
			HandleUpsertSession(ctx, s)
			continue
		}
		s.Monitors = &sync.Map{}
		onlineSession.LoadOrStore(s.SessionId, s)
	}

	return
}

func HandleUpsertSession(ctx *gin.Context, data *Session) (err error) {
	if err = mysql.DB.
		Clauses(clause.OnConflict{
			DoUpdates: clause.AssignmentColumns([]string{"status", "closed_at"}),
		}).
		Create(data).
		Error; err != nil {
		return
	}

	switch data.Status {
	case model.SESSIONSTATUS_ONLINE:
		if data.Monitors == nil {
			data.Monitors = &sync.Map{}
		}
		_, ok := onlineSession.LoadOrStore(data.SessionId, data)
		if ok {
			err = fmt.Errorf("failed to loadstore online session")
		}
	case model.SESSIONSTATUS_OFFLINE:
		// doOfflineOnlineSession(ctx, data.SessionId, "")
	}

	return
}
