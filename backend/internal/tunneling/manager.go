package tunneling

import "github.com/veops/oneterm/internal/model"

// Global tunnel manager instance
var DefaultManager = NewTunnelManager()

// GetTunnelBySessionId gets a gateway tunnel by session ID using the default manager
func GetTunnelBySessionId(sessionId string) *GatewayTunnel {
	return DefaultManager.GetTunnelBySessionId(sessionId)
}

// OpenTunnel opens a new gateway tunnel using the default manager
func OpenTunnel(isConnectable bool, sessionId, remoteIp string, remotePort int, gateway *model.Gateway) (*GatewayTunnel, error) {
	return DefaultManager.OpenTunnel(isConnectable, sessionId, remoteIp, remotePort, gateway)
}

// CloseTunnels closes gateway tunnels by session IDs using the default manager
func CloseTunnels(sessionIds ...string) {
	DefaultManager.CloseTunnels(sessionIds...)
}
