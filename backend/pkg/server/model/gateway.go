package model

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/spf13/cast"
	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/plugin/soft_delete"
)

type Gateway struct {
	Id          int    `json:"id" gorm:"column:id;primarykey"`
	Name        string `json:"name" gorm:"column:name"`
	Host        string `json:"host" gorm:"column:host"`
	Port        int    `json:"port" gorm:"column:port"`
	AccountType int    `json:"account_type" gorm:"column:account_type"`
	Account     string `json:"account" gorm:"column:account"`
	Password    string `json:"password" gorm:"column:password"`
	Pk          string `json:"pk" gorm:"column:pk"`
	Phrase      string `json:"phrase" gorm:"column:phrase"`

	ResourceId int                   `json:"resource_id" gorm:"column:resource_id"`
	CreatorId  int                   `json:"creator_id" gorm:"column:creator_id"`
	UpdaterId  int                   `json:"updater_id" gorm:"column:updater_id"`
	CreatedAt  time.Time             `json:"created_at" gorm:"column:created_at"`
	UpdatedAt  time.Time             `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt  soft_delete.DeletedAt `json:"-" gorm:"column:deleted_at"`

	AssetCount int64 `json:"asset_count" gorm:"-"`
}

func (m *Gateway) TableName() string {
	return "gateway"
}
func (m *Gateway) SetId(id int) {
	m.Id = id
}
func (m *Gateway) SetCreatorId(creatorId int) {
	m.CreatorId = creatorId
}
func (m *Gateway) SetUpdaterId(updaterId int) {
	m.UpdaterId = updaterId
}
func (m *Gateway) SetResourceId(resourceId int) {
	m.ResourceId = resourceId
}
func (m *Gateway) GetResourceId() int {
	return m.ResourceId
}
func (m *Gateway) GetName() string {
	return m.Name
}
func (m *Gateway) GetId() int {
	return m.Id
}

type GatewayCount struct {
	Id    int   `gorm:"column:id"`
	Count int64 `gorm:"column:count"`
}

type gatewayTunnelKey [3]string

type GatewayTunnel struct {
	Key               gatewayTunnelKey
	LocalIp           string
	LocalPort         int
	RemoteIp          string
	RemotePort        int
	listener          net.Listener
	localConnections  map[string]net.Conn
	remoteConnections map[string]net.Conn
	sshClient         *ssh.Client
	using             bool
}

func (gt *GatewayTunnel) Open(sessionId, remoteIp string, remotePort int) error {
	for {
		lc, err := gt.listener.Accept()
		if err != nil {
			logger.L.Error("accept failed", zap.Error(err))
			return err
		}
		gt.localConnections[sessionId] = lc

		remoteAddr := fmt.Sprintf("%s:%d", remoteIp, remotePort)
		rc, err := gt.sshClient.Dial("tcp", remoteAddr)
		if err != nil {
			logger.L.Error("dial remote failed", zap.Error(err))
			return err
		}
		gt.remoteConnections[sessionId] = rc

		go io.Copy(lc, rc)
		go io.Copy(rc, lc)
	}
}

func (gt *GatewayTunnel) Close(sessionId string) {
	if c, ok := gt.remoteConnections[sessionId]; ok {
		c.Close()
	}
	delete(gt.localConnections, sessionId)

	if c, ok := gt.localConnections[sessionId]; ok {
		c.Close()
	}
	delete(gt.remoteConnections, sessionId)

	gt.using = len(gt.localConnections) > 0 && len(gt.remoteConnections) > 0
}

type GateWayManager struct {
	gateways map[gatewayTunnelKey]*GatewayTunnel
	mtx      sync.Mutex
}

func NewGateWayManager() *GateWayManager {
	return &GateWayManager{
		gateways: map[gatewayTunnelKey]*GatewayTunnel{},
		mtx:      sync.Mutex{},
	}
}

func (gm *GateWayManager) Open(sessionId, remoteIp string, remotePort int, gateway *Gateway) (g *GatewayTunnel, err error) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	key := gatewayTunnelKey{cast.ToString(gateway.Id), remoteIp, cast.ToString(remotePort)}
	g, ok := gm.gateways[key]
	if ok {
		return
	}
	g = &GatewayTunnel{}

	auth, err := gm.getAuth(gateway)
	if err != nil {
		return
	}
	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port), &ssh.ClientConfig{
		User:            gateway.Account,
		Auth:            []ssh.AuthMethod{auth},
		Timeout:         time.Second * 3,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return
	}

	localPort, err := getAvailablePort()
	if err != nil {
		return
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", conf.Cfg.Guacd.Gateway, localPort))
	if err != nil {
		return
	}
	g = &GatewayTunnel{
		Key:               key,
		LocalIp:           conf.Cfg.Guacd.Gateway,
		LocalPort:         localPort,
		listener:          listener,
		localConnections:  map[string]net.Conn{},
		remoteConnections: map[string]net.Conn{},
		sshClient:         sshClient,
		using:             true,
	}
	gm.gateways[key] = g
	go g.Open(sessionId, remoteIp, remotePort)

	return
}

func (gm *GateWayManager) Close(key gatewayTunnelKey, sessionId string) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	g, ok := gm.gateways[key]
	if ok {
		g.Close(sessionId)
	}
	if !g.using {
		defer g.sshClient.Close()
		delete(gm.gateways, key)
	}
}

func (gm *GateWayManager) getAuth(gateway *Gateway) (ssh.AuthMethod, error) {
	switch gateway.AccountType {
	case AUTHMETHOD_PASSWORD:
		return ssh.Password(gateway.Password), nil
	case AUTHMETHOD_PUBLICKEY:
		if gateway.Phrase == "" {
			pk, err := ssh.ParsePrivateKey([]byte(gateway.Pk))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		} else {
			pk, err := ssh.ParsePrivateKeyWithPassphrase([]byte(gateway.Pk), []byte(gateway.Phrase))
			if err != nil {
				return nil, err
			}
			return ssh.PublicKeys(pk), nil
		}
	default:
		return nil, fmt.Errorf("invalid authmethod %d", gateway.AccountType)
	}
}

func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}
