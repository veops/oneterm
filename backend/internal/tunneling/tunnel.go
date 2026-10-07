package tunneling

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/singleflight"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/sshclient"
)

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
	mu         sync.Mutex
	closed     bool
	client     *ssh.Client
	ctx        context.Context
	cancel     context.CancelFunc
}

func (gt *GatewayTunnel) close() {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	gt.closed = true
	if gt.cancel != nil {
		gt.cancel()
	}
	if gt.listener != nil {
		gt.listener.Close()
	}
	if gt.LocalConn != nil {
		gt.LocalConn.Close()
	}
	if gt.RemoteConn != nil {
		gt.RemoteConn.Close()
	}
}

func (gt *GatewayTunnel) attach(connection net.Conn, remote bool) bool {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if gt.closed {
		connection.Close()
		return false
	}
	if remote {
		gt.RemoteConn = connection
	} else {
		gt.LocalConn = connection
	}
	return true
}

func (gt *GatewayTunnel) Open(client *ssh.Client, probe bool) (err error) {
	defer gt.close()
	defer func() {
		select {
		case gt.Opened <- err:
		default:
		}
	}()
	timer := time.AfterFunc(3*time.Second, func() { gt.listener.Close() })
	local, err := gt.listener.Accept()
	timer.Stop()
	gt.listener.Close()
	if err != nil {
		return err
	}
	if !gt.attach(local, false) {
		return context.Canceled
	}
	ctx, cancel := context.WithTimeout(gt.ctx, 10*time.Second)
	defer cancel()
	remote, err := client.DialContext(ctx, "tcp", net.JoinHostPort(gt.RemoteIp, strconv.Itoa(gt.RemotePort)))
	if err != nil {
		return err
	}
	if !gt.attach(remote, true) {
		return context.Canceled
	}
	if probe {
		return nil
	}
	gt.Opened <- nil
	done := make(chan struct{})
	go func() { defer close(done); io.Copy(remote, local); gt.close() }()
	_, err = io.Copy(local, remote)
	gt.close()
	<-done
	return err
}

type TunnelManager struct {
	gatewayTunnels  map[string]*GatewayTunnel
	sshClients      map[int]*ssh.Client
	sshClientsCount map[int]int
	mtx             sync.RWMutex
	dials           singleflight.Group
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{gatewayTunnels: map[string]*GatewayTunnel{}, sshClients: map[int]*ssh.Client{}, sshClientsCount: map[int]int{}}
}

func (tm *TunnelManager) GetTunnelBySessionId(id string) *GatewayTunnel {
	tm.mtx.RLock()
	defer tm.mtx.RUnlock()
	return tm.gatewayTunnels[id]
}

func (tm *TunnelManager) gatewayClient(gateway *model.Gateway) (*ssh.Client, error) {
	for {
		tm.mtx.Lock()
		if client := tm.sshClients[gateway.Id]; client != nil {
			tm.sshClientsCount[gateway.Id]++
			tm.mtx.Unlock()
			return client, nil
		}
		tm.mtx.Unlock()
		value, err, _ := tm.dials.Do(strconv.Itoa(gateway.Id), func() (any, error) {
			tm.mtx.RLock()
			client := tm.sshClients[gateway.Id]
			tm.mtx.RUnlock()
			if client != nil {
				return client, nil
			}
			auth, err := tm.getAuthMethod(gateway)
			if err != nil {
				return nil, err
			}
			address := net.JoinHostPort(gateway.Host, strconv.Itoa(gateway.Port))
			client, err = sshclient.Dial(context.Background(), address, &ssh.ClientConfig{User: gateway.Account,
				Auth: []ssh.AuthMethod{auth}, Timeout: 10 * time.Second, HostKeyCallback: sshclient.HostKey(address)})
			if err != nil {
				return nil, err
			}
			tm.mtx.Lock()
			tm.sshClients[gateway.Id] = client
			tm.mtx.Unlock()
			go func() {
				client.Wait()
				tm.mtx.Lock()
				if tm.sshClients[gateway.Id] == client {
					delete(tm.sshClients, gateway.Id)
					delete(tm.sshClientsCount, gateway.Id)
				}
				tm.mtx.Unlock()
			}()
			return client, nil
		})
		if err != nil {
			return nil, err
		}
		client := value.(*ssh.Client)
		tm.mtx.Lock()
		if tm.sshClients[gateway.Id] == client {
			tm.sshClientsCount[gateway.Id]++
			tm.mtx.Unlock()
			return client, nil
		}
		tm.mtx.Unlock()
	}
}

func (tm *TunnelManager) OpenTunnel(probe bool, id, host string, port int, gateway *model.Gateway) (*GatewayTunnel, error) {
	if gateway == nil {
		return nil, fmt.Errorf("gateway is nil")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	client, err := tm.gatewayClient(gateway)
	if err != nil {
		listener.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	tunnel := &GatewayTunnel{listener: listener, GatewayId: gateway.Id, SessionId: id, LocalIp: "127.0.0.1",
		LocalPort: listener.Addr().(*net.TCPAddr).Port, RemoteIp: host, RemotePort: port, Opened: make(chan error, 1), client: client, ctx: ctx, cancel: cancel}
	tm.CloseTunnels(id)
	tm.mtx.Lock()
	tm.gatewayTunnels[id] = tunnel
	tm.mtx.Unlock()
	go tunnel.Open(client, probe)
	return tunnel, nil
}

func (tm *TunnelManager) CloseTunnels(ids ...string) {
	var tunnels []*GatewayTunnel
	var clients []*ssh.Client
	tm.mtx.Lock()
	for _, id := range ids {
		tunnel := tm.gatewayTunnels[id]
		if tunnel == nil {
			continue
		}
		delete(tm.gatewayTunnels, id)
		tunnels = append(tunnels, tunnel)
		if tm.sshClients[tunnel.GatewayId] == tunnel.client {
			tm.sshClientsCount[tunnel.GatewayId]--
			if tm.sshClientsCount[tunnel.GatewayId] <= 0 {
				clients = append(clients, tunnel.client)
				delete(tm.sshClients, tunnel.GatewayId)
				delete(tm.sshClientsCount, tunnel.GatewayId)
			}
		}
	}
	tm.mtx.Unlock()
	for _, tunnel := range tunnels {
		tunnel.close()
	}
	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}
}

func (tm *TunnelManager) getAuthMethod(gateway *model.Gateway) (ssh.AuthMethod, error) {
	switch gateway.AccountType {
	case model.AUTHMETHOD_PASSWORD:
		return ssh.Password(gateway.Password), nil
	case model.AUTHMETHOD_PUBLICKEY:
		var signer ssh.Signer
		var err error
		if gateway.Phrase == "" {
			signer, err = ssh.ParsePrivateKey([]byte(gateway.Pk))
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(gateway.Pk), []byte(gateway.Phrase))
		}
		if err != nil {
			return nil, err
		}
		return ssh.PublicKeys(signer), nil
	}
	return nil, fmt.Errorf("invalid authmethod %d", gateway.AccountType)
}

func Target(protocol string, asset *model.Asset) (string, int, error) {
	host, addressPort := strings.TrimSpace(asset.Ip), 0
	if parsed, port, err := net.SplitHostPort(host); err == nil {
		host = parsed
		addressPort, _ = strconv.Atoi(port)
	} else if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", 0, fmt.Errorf("invalid target address")
	}
	if host == "" {
		return "", 0, fmt.Errorf("empty target address")
	}
	for _, wanted := range strings.Split(protocol, ",") {
		name, explicit, hasPort := strings.Cut(wanted, ":")
		for _, configured := range asset.Protocols {
			kind, value, _ := strings.Cut(configured, ":")
			if kind != name || hasPort && value != explicit {
				continue
			}
			port, err := strconv.Atoi(value)
			if !hasPort && addressPort != 0 {
				port = addressPort
				err = nil
			}
			if err == nil && port > 0 && port <= 65535 {
				return host, port, nil
			}
		}
	}
	return "", 0, fmt.Errorf("target protocol is not configured")
}

func Proxy(probe bool, id, protocol string, asset *model.Asset, gateway *model.Gateway) (string, int, error) {
	host, port, err := Target(protocol, asset)
	if err != nil || asset.GatewayId == 0 || gateway == nil {
		return host, port, err
	}
	tunnel, err := OpenTunnel(probe, id, host, port, gateway)
	if err != nil {
		return "", 0, err
	}
	return tunnel.LocalIp, tunnel.LocalPort, nil
}
