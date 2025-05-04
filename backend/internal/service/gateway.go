package service

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/utils"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// GatewayService handles gateway business logic
type GatewayService struct {
	repo          repository.GatewayRepository
	tunnelManager *tunneling.TunnelManager
}

// NewGatewayService creates a new gateway service
func NewGatewayService() *GatewayService {
	return &GatewayService{
		repo:          repository.NewGatewayRepository(),
		tunnelManager: tunneling.NewTunnelManager(),
	}
}

// ValidatePublicKey validates the given public key
func (s *GatewayService) ValidatePublicKey(gateway *model.Gateway) error {
	if gateway.AccountType != model.AUTHMETHOD_PUBLICKEY {
		return nil
	}

	var err error
	if gateway.Phrase == "" {
		_, err = ssh.ParsePrivateKey([]byte(gateway.Pk))
	} else {
		_, err = ssh.ParsePrivateKeyWithPassphrase([]byte(gateway.Pk), []byte(gateway.Phrase))
	}
	return err
}

// EncryptSensitiveData encrypts sensitive gateway data
func (s *GatewayService) EncryptSensitiveData(gateway *model.Gateway) {
	gateway.Password = utils.EncryptAES(gateway.Password)
	gateway.Pk = utils.EncryptAES(gateway.Pk)
	gateway.Phrase = utils.EncryptAES(gateway.Phrase)
}

// DecryptSensitiveData decrypts sensitive gateway data
func (s *GatewayService) DecryptSensitiveData(gateways []*model.Gateway) {
	for _, g := range gateways {
		g.Password = utils.DecryptAES(g.Password)
		g.Pk = utils.DecryptAES(g.Pk)
		g.Phrase = utils.DecryptAES(g.Phrase)
	}
}

// AttachAssetCount attaches asset count to gateways
func (s *GatewayService) AttachAssetCount(ctx context.Context, gateways []*model.Gateway) error {
	return s.repo.AttachAssetCount(ctx, gateways)
}

// CheckAssetDependencies checks if gateway has dependent assets
func (s *GatewayService) CheckAssetDependencies(ctx context.Context, id int) (string, error) {
	return s.repo.CheckAssetDependencies(ctx, id)
}

// BuildQuery constructs gateway query with basic filters
func (s *GatewayService) BuildQuery(ctx *gin.Context) *gorm.DB {
	return s.repo.BuildQuery(ctx)
}

// FilterByAssetIds filters gateways by related asset IDs
func (s *GatewayService) FilterByAssetIds(db *gorm.DB, assetIds []int) *gorm.DB {
	return s.repo.FilterByAssetIds(db, assetIds)
}

// GetTunnelBySessionId gets a gateway tunnel by session ID
func (s *GatewayService) GetTunnelBySessionId(sessionId string) *tunneling.GatewayTunnel {
	return s.tunnelManager.GetTunnelBySessionId(sessionId)
}

// OpenTunnel opens a new gateway tunnel
func (s *GatewayService) OpenTunnel(isConnectable bool, sessionId, remoteIp string, remotePort int, gateway *model.Gateway) (*tunneling.GatewayTunnel, error) {
	return s.tunnelManager.OpenTunnel(isConnectable, sessionId, remoteIp, remotePort, gateway)
}

// CloseTunnels closes gateway tunnels by session IDs
func (s *GatewayService) CloseTunnels(sessionIds ...string) {
	s.tunnelManager.CloseTunnels(sessionIds...)
}
