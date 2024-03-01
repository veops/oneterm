package guacd

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/model"
)

var (
	GlobalGatewayManager = &gateWayManager{
		gateways: map[GatewayTunnelKey]*gatewayTunnel{},
		mtx:      sync.Mutex{},
	}
)

type GatewayTunnelKey [3]string

type gatewayTunnel struct {
	Key               GatewayTunnelKey
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

func (gt *gatewayTunnel) Open(sessionId, remoteIp string, remotePort int) error {
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

func (gt *gatewayTunnel) Close(sessionId string) {
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

type gateWayManager struct {
	gateways map[GatewayTunnelKey]*gatewayTunnel
	mtx      sync.Mutex
}

func (gm *gateWayManager) Open(sessionId, remoteIp string, remotePort int, gateway *model.Gateway) (g *gatewayTunnel, err error) {
	if gateway == nil {
		err = fmt.Errorf("gateway is nil")
		return
	}
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	key := GatewayTunnelKey{cast.ToString(gateway.Id), remoteIp, cast.ToString(remotePort)}
	g, ok := gm.gateways[key]
	if ok {
		return
	}
	g = &gatewayTunnel{}

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
	g = &gatewayTunnel{
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
	logger.L.Debug("opening gateway", zap.Any("key", key))
	go g.Open(sessionId, remoteIp, remotePort)

	return
}

func (gm *gateWayManager) Close(key GatewayTunnelKey, sessionIds ...string) {
	gm.mtx.Lock()
	defer gm.mtx.Unlock()

	g, ok := gm.gateways[key]
	if ok {
		for _, sid := range sessionIds {
			g.Close(sid)
		}
	}
	if !g.using {
		logger.L.Debug("closing gateway", zap.Any("key", key))
		defer g.sshClient.Close()
		delete(gm.gateways, key)
	}
}

func (gm *gateWayManager) getAuth(gateway *model.Gateway) (ssh.AuthMethod, error) {
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
