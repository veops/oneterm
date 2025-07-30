package protocols

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/logger"
)

type WebSession struct {
	Asset        *model.Asset
	Account      *model.Account
	StartTime    time.Time
	LastActivity time.Time
}

// NewWebSession creates a new web session
func NewWebSession(asset *model.Asset, account *model.Account) *WebSession {
	now := time.Now()
	return &WebSession{
		Asset:        asset,
		Account:      account,
		StartTime:    now,
		LastActivity: now,
	}
}

// GetTargetURL returns the target URL for the web asset
func (ws *WebSession) GetTargetURL() string {
	if ws.Asset == nil {
		return ""
	}

	protocol, port := ws.Asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	// Build URL without port if it's the default port
	if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
		return fmt.Sprintf("%s://%s", protocol, ws.Asset.Ip)
	}

	return fmt.Sprintf("%s://%s:%d", protocol, ws.Asset.Ip, port)
}

// GetAssetInfo returns asset information
func (ws *WebSession) GetAssetInfo() map[string]interface{} {
	if ws.Asset == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"id":   ws.Asset.Id,
		"name": ws.Asset.Name,
		"ip":   ws.Asset.Ip,
		"url":  ws.GetTargetURL(),
	}
}

// Close closes the web session
func (ws *WebSession) Close() error {
	logger.L().Info("Closing web session",
		zap.String("assetName", ws.Asset.Name),
		zap.Int("assetId", ws.Asset.Id))
	return nil
}
