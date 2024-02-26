package model

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

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

type GatewayTunnel struct {
	LocalIp           string
	LocalPort         int
	listener          net.Listener
	localConnections  map[string]net.Conn
	remoteConnections map[string]net.Conn
	sshClient         *ssh.Client
}

func (gt *GatewayTunnel) Open(sessionId, remoteIp string, remotePort int) error {
	for {
		lc, err := gt.listener.Accept()
		if err != nil {
			return err
		}
		gt.localConnections[sessionId] = lc

		remoteAddr := fmt.Sprintf("%s:%d", remoteIp, remotePort)
		rc, err := gt.sshClient.Dial("tcp", remoteAddr)
		if err != nil {
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
	if c, ok := gt.localConnections[sessionId]; ok {
		c.Close()
	}
	
}

type GateWayManager struct {
	gateways map[int]*GatewayTunnel
	mtx      sync.Mutex
}

func (gm *GateWayManager) Open(sessionId, remoteIp string, remotePort int, gateway *Gateway) (g *GatewayTunnel, err error) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	g, ok := gm.gateways[gateway.Id]
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
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return
	}
	g = &GatewayTunnel{
		LocalIp:           "127.0.0.1",
		LocalPort:         localPort,
		listener:          listener,
		localConnections:  map[string]net.Conn{},
		remoteConnections: map[string]net.Conn{},
		sshClient:         sshClient,
	}
	err = g.Open(sessionId, remoteIp, remotePort)

	return
}

func (gm *GateWayManager) Close(id int) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	g, ok := gm.gateways[id]
	if ok {
		g.Close()
	}
	delete(gm.gateways, id)
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

	defer func(l *net.TCPListener) {
		_ = l.Close()
	}(l)
	return l.Addr().(*net.TCPAddr).Port, nil
}
