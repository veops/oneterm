package gateway

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/conf"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

var (
	manager = &GateWayManager{
		gateways:        map[string]*GatewayTunnel{},
		sshClients:      map[int]*ssh.Client{},
		sshClientsCount: map[int]int{},
		mtx:             sync.Mutex{},
	}
)

func GetGatewayManager() *GateWayManager {
	return manager
}

type GatewayTunnel struct {
	listener   net.Listener
	GatewayId  int
	SessionId  string
	LocalIp    string
	LocalPort  int
	RemoteIp   string
	RemotePort int
	LocalConn  net.Conn
	RemoteConn net.Conn
	Chan       chan struct{}
}

func (gt *GatewayTunnel) Open() (err error) {
	defer close(gt.Chan)
	go func() {
		<-time.After(time.Second * 5)
		logger.L().Debug("timeout 5 second close listener", zap.String("sessionId", gt.SessionId))
		gt.listener.Close()
	}()
	gt.LocalConn, err = gt.listener.Accept()
	if err != nil {
		logger.L().Error("accept failed", zap.String("sessionId", gt.SessionId), zap.Error(err))
		return err
	}

	remoteAddr := fmt.Sprintf("%s:%d", gt.RemoteIp, gt.RemotePort)
	gt.RemoteConn, err = manager.sshClients[gt.GatewayId].Dial("tcp", remoteAddr)
	if err != nil {
		logger.L().Error("dial remote failed", zap.String("sessionId", gt.SessionId), zap.Error(err))
		return err
	}

	go io.Copy(gt.LocalConn, gt.RemoteConn)
	go io.Copy(gt.RemoteConn, gt.LocalConn)

	return
}

type GateWayManager struct {
	gateways        map[string]*GatewayTunnel
	sshClients      map[int]*ssh.Client
	sshClientsCount map[int]int
	mtx             sync.Mutex
}

func (gm *GateWayManager) Open(sessionId, remoteIp string, remotePort int, gateway *model.Gateway) (g *GatewayTunnel, err error) {
	if gateway == nil {
		err = fmt.Errorf("gateway is nil")
		return
	}
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	sshCli, ok := gm.sshClients[gateway.Id]
	if !ok {
		var auth ssh.AuthMethod
		auth, err = gm.getAuth(gateway)
		if err != nil {
			return
		}
		sshCli, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port), &ssh.ClientConfig{
			User:            gateway.Account,
			Auth:            []ssh.AuthMethod{auth},
			Timeout:         time.Second * 3,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err != nil {
			return
		}
	}
	gm.sshClients[gateway.Id] = sshCli
	gm.sshClientsCount[gateway.Id] += 1
	localPort, err := getAvailablePort()
	if err != nil {
		return
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", conf.Cfg.Guacd.Gateway, localPort))
	if err != nil {
		return
	}
	g = &GatewayTunnel{
		listener:   listener,
		GatewayId:  gateway.Id,
		SessionId:  sessionId,
		LocalIp:    conf.Cfg.Guacd.Gateway,
		LocalPort:  localPort,
		RemoteIp:   remoteIp,
		RemotePort: remotePort,
		Chan:       make(chan struct{}),
	}
	gm.gateways[sessionId] = g
	logger.L().Debug("opening gateway", zap.Any("sessionId", sessionId))
	go g.Open()

	return
}

func (gm *GateWayManager) Close(sessionIds ...string) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()
	for _, sid := range sessionIds {
		gt, ok := gm.gateways[sid]
		if !ok {
			return
		}
		gm.sshClientsCount[gt.GatewayId] -= 1
		if gm.sshClientsCount[gt.GatewayId] <= 0 {
			gm.sshClients[gt.GatewayId].Close()
			delete(gm.sshClients, gt.GatewayId)
			delete(gm.sshClientsCount, gt.GatewayId)
		}
	}
}

func (gm *GateWayManager) getAuth(gateway *model.Gateway) (ssh.AuthMethod, error) {
	switch gateway.AccountType {
	case model.AUTHMETHOD_PASSWORD:
		return ssh.Password(gateway.Password), nil
	case model.AUTHMETHOD_PUBLICKEY:
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
