package model

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	SESSIONTYPE_WEB = iota + 1
	SESSIONTYPE_CLIENT
)

const (
	SESSIONSTATUS_ONLINE = iota + 1
	SESSIONSTATUS_OFFLINE
)

const (
	SESSIONACTION_NEW = iota + 1
	SESSIONACTION_MONITOR
	SESSIONACTION_CLOSE
)

type Session struct {
	Id          int        `json:"id" gorm:"column:id;primarykey"`
	SessionType int        `json:"session_type" gorm:"column:session_type"`
	SessionId   string     `json:"session_id" gorm:"column:session_id"`
	Uid         int        `json:"uid" gorm:"column:uid"`
	UserName    string     `json:"user_name" gorm:"column:user_name"`
	AssetId     int        `json:"asset_id" gorm:"column:asset_id"`
	AssetInfo   string     `json:"asset_info" gorm:"column:asset_info"`
	AccountId   int        `json:"account_id" gorm:"column:account_id"`
	AccountInfo string     `json:"account_info" gorm:"column:account_info"`
	GatewayId   int        `json:"gateway_id" gorm:"column:gateway_id"`
	GatewayInfo string     `json:"gateway_info" gorm:"column:gateway_info"`
	ClientIp    string     `json:"client_ip" gorm:"column:client_ip"`
	Protocol    string     `json:"protocol" gorm:"column:protocol"`
	Status      int        `json:"status" gorm:"column:status"`
	Duration    int64      `json:"duration" gorm:"-"`
	ClosedAt    *time.Time `json:"closed_at" gorm:"column:closed_at"`

	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`

	CmdCount int64 `json:"cmd_count" gorm:"-"`

	Monitors     *sync.Map     `json:"-" gorm:"-"`
	Chans        *SessionChans `json:"-" gorm:"-"`
	Connected    atomic.Bool   `json:"-" gorm:"-"`
	ConnectionId string        `json:"-" gorm:"-"`
}

func (m *Session) TableName() string {
	return "session"
}

func (m *Session) IsSsh() bool {
	return strings.HasPrefix(m.Protocol, "ssh")
}

func (m *Session) HasMonitors() (has bool) {
	m.Monitors.Range(func(key, value any) bool {
		has = true
		return false
	})
	return
}

type SessionCmd struct {
	Id        int    `json:"id" gorm:"column:id;primarykey"`
	SessionId string `json:"session_id" gorm:"column:session_id"`
	Cmd       string `json:"cmd" gorm:"column:cmd"`
	Result    string `json:"result" gorm:"column:result"`
	Level     int    `json:"level" gorm:"column:level"`

	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

func (m *SessionCmd) TableName() string {
	return "session_cmd"
}

type CmdCount struct {
	SessionId string `gorm:"column:session_id"`
	Count     int64  `gorm:"column:count"`
}

type SessionOptionAsset struct {
	Id   int    `json:"id" gorm:"column:id;primarykey"`
	Name string `json:"name" gorm:"column:name"`
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

type SessionManager struct {
}
