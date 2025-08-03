package tunneling

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/logger"
)

// GatewayTunnel represents a SSH tunnel through a gateway
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
	Opened     chan error
}

// Open opens the gateway tunnel
func (gt *GatewayTunnel) Open(sshClient *ssh.Client, isConnectable bool) (err error) {
	go func() {
		<-time.After(time.Second * 3)
		logger.L().Debug("timeout 3 second close listener", zap.String("sessionId", gt.SessionId))
		gt.listener.Close()
	}()
	defer func() {
		logger.L().Debug("close listener", zap.String("sessionId", gt.SessionId), zap.Error(err))
		gt.Opened <- err
	}()
	gt.Opened <- nil
	gt.LocalConn, err = gt.listener.Accept()
	if err != nil {
		logger.L().Error("accept failed", zap.String("sessionId", gt.SessionId), zap.Error(err))
		return
	}
	remoteAddr := fmt.Sprintf("%s:%d", gt.RemoteIp, gt.RemotePort)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	gt.RemoteConn, err = sshClient.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		defer func() {
			if gt.LocalConn != nil {
				defer gt.LocalConn.Close()
			}
			if gt.RemoteConn != nil {
				defer gt.RemoteConn.Close()
			}
		}()
		logger.L().Error("dial remote failed", zap.String("sessionId", gt.SessionId), zap.Error(err))
		return
	}
	if isConnectable {
		return
	}
	go io.Copy(gt.LocalConn, gt.RemoteConn)
	go io.Copy(gt.RemoteConn, gt.LocalConn)

	return
}

// TunnelManager manages SSH tunnels through gateways
type TunnelManager struct {
	gatewayTunnels  map[string]*GatewayTunnel
	sshClients      map[int]*ssh.Client
	sshClientsCount map[int]int
	mtx             sync.Mutex
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		gatewayTunnels:  map[string]*GatewayTunnel{},
		sshClients:      map[int]*ssh.Client{},
		sshClientsCount: map[int]int{},
		mtx:             sync.Mutex{},
	}
}

// GetTunnelBySessionId gets a gateway tunnel by session ID
func (tm *TunnelManager) GetTunnelBySessionId(sessionId string) *GatewayTunnel {
	return tm.gatewayTunnels[sessionId]
}

// OpenTunnel opens a new gateway tunnel
func (tm *TunnelManager) OpenTunnel(isConnectable bool, sessionId, remoteIp string, remotePort int, gateway *model.Gateway) (*GatewayTunnel, error) {
	if gateway == nil {
		return nil, fmt.Errorf("gateway is nil")
	}
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	sshCli, ok := tm.sshClients[gateway.Id]
	if !ok {
		auth, err := tm.getAuthMethod(gateway)
		if err != nil {
			return nil, err
		}

		sshCli, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port), &ssh.ClientConfig{
			User:            gateway.Account,
			Auth:            []ssh.AuthMethod{auth},
			Timeout:         time.Second,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err != nil {
			logger.L().Error("open gateway sshcli failed", zap.Int("gatewayId", gateway.Id), zap.Error(err))
			return nil, err
		}
		go func() {
			logger.L().Debug("ssh proxy wait closed", zap.Int("gatewayId", gateway.Id), zap.Error(sshCli.Wait()))
			delete(tm.sshClients, gateway.Id)
		}()
	}
	tm.sshClients[gateway.Id] = sshCli
	tm.sshClientsCount[gateway.Id] += 1

	localPort, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", localPort))
	if err != nil {
		return nil, err
	}

	g := &GatewayTunnel{
		listener:   listener,
		GatewayId:  gateway.Id,
		SessionId:  sessionId,
		LocalIp:    "localhost",
		LocalPort:  localPort,
		RemoteIp:   remoteIp,
		RemotePort: remotePort,
		Opened:     make(chan error),
	}
	tm.gatewayTunnels[sessionId] = g

	go g.Open(sshCli, isConnectable)

	logger.L().Debug("opening gateway", zap.Any("sessionId", sessionId))
	<-g.Opened
	logger.L().Debug("opened gateway", zap.Any("sessionId", sessionId))

	return g, nil
}

// CloseTunnels closes gateway tunnels by session IDs
func (tm *TunnelManager) CloseTunnels(sessionIds ...string) {
	tm.mtx.Lock()
	defer tm.mtx.Unlock()

	for _, sid := range sessionIds {
		gt, ok := tm.gatewayTunnels[sid]
		if !ok {
			continue
		}

		tm.sshClientsCount[gt.GatewayId] -= 1
		if tm.sshClientsCount[gt.GatewayId] <= 0 {
			if g := tm.sshClients[gt.GatewayId]; g != nil {
				g.Close()
			}
			delete(tm.sshClients, gt.GatewayId)
			delete(tm.sshClientsCount, gt.GatewayId)
		}

		// Close and delete tunnel
		if gt.listener != nil {
			gt.listener.Close()
		}
		if gt.LocalConn != nil {
			gt.LocalConn.Close()
		}
		if gt.RemoteConn != nil {
			gt.RemoteConn.Close()
		}
		delete(tm.gatewayTunnels, sid)
	}
}

// getAuthMethod gets SSH authentication method based on gateway config
func (tm *TunnelManager) getAuthMethod(gateway *model.Gateway) (ssh.AuthMethod, error) {
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

// getAvailablePort gets an available local port
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

// Proxy establishes a proxy connection to an asset through a gateway if necessary
func Proxy(isConnectable bool, sessionId string, protocol string, asset *model.Asset, gateway *model.Gateway) (ip string, port int, err error) {
	// Handle case 1: asset.Ip already contains port (e.g., "127.0.0.1:8000")
	if strings.Contains(asset.Ip, ":") {
		ipParts := strings.Split(asset.Ip, ":")
		if len(ipParts) >= 2 {
			ip = ipParts[0]
			port = cast.ToInt(ipParts[1])
		} else {
			ip = asset.Ip
			port = 0
		}
	} else {
		// Case 2: asset.Ip without port (e.g., "127.0.0.1"), extract port from protocol
		ip, port = asset.Ip, 0
		for _, tp := range strings.Split(protocol, ",") {
			for _, p := range asset.Protocols {
				if strings.HasPrefix(strings.ToLower(p), tp) {
					parts := strings.Split(p, ":")
					if len(parts) >= 2 {
						if port = cast.ToInt(parts[1]); port != 0 {
							break
						}
					}
				}
			}
		}
	}

	if asset.GatewayId == 0 || gateway == nil {
		return
	}

	g, err := OpenTunnel(isConnectable, sessionId, ip, port, gateway)
	if err != nil {
		return
	}
	ip, port = g.LocalIp, g.LocalPort
	return
}
