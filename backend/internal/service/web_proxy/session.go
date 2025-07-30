package web_proxy

import (
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/logger"
)

// Global session storage
var webProxySessions = make(map[string]*WebProxySession)

// WebProxySession represents an active web proxy session
type WebProxySession struct {
	SessionId    string
	AssetId      int
	AccountId    int
	Asset        *model.Asset
	CreatedAt    time.Time
	LastActivity time.Time
	CurrentHost  string
	Permissions  *model.AuthPermissions // User permissions for this asset
	WebConfig    *model.WebConfig       // Web-specific configuration
}

// cleanupExpiredSessions removes inactive sessions from storage
func cleanupExpiredSessions(maxInactiveTime time.Duration) {
	now := time.Now()
	for sessionID, session := range webProxySessions {
		if now.Sub(session.LastActivity) > maxInactiveTime {
			delete(webProxySessions, sessionID)
			logger.L().Info("Cleaned up expired web session",
				zap.String("sessionID", sessionID))
		}
	}
}

// StartSessionCleanupRoutine starts background cleanup routine for web sessions
func StartSessionCleanupRoutine() {
	// More frequent cleanup - every 30 seconds to catch closed browser tabs quickly
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			// For web sessions, clean up after 2 minutes of inactivity (browser likely closed)
			webInactiveTime := 2 * time.Minute
			cleanupExpiredSessions(webInactiveTime)
		}
	}()
}

// GetSession retrieves a session by ID
func GetSession(sessionID string) (*WebProxySession, bool) {
	session, exists := webProxySessions[sessionID]
	return session, exists
}

// StoreSession stores a session in the session map
func StoreSession(sessionID string, session *WebProxySession) {
	webProxySessions[sessionID] = session
}

// DeleteSession removes a session from the session map
func DeleteSession(sessionID string) {
	delete(webProxySessions, sessionID)
}

// UpdateSessionActivity updates the last activity time for a session
func UpdateSessionActivity(sessionID string) {
	if session, exists := webProxySessions[sessionID]; exists {
		session.LastActivity = time.Now()
	}
}

// UpdateSessionHost updates the current host for a session
func UpdateSessionHost(sessionID string, host string) {
	if session, exists := webProxySessions[sessionID]; exists {
		session.CurrentHost = host
	}
}

// GetActiveSessionsForAsset returns the number of active sessions for an asset
func GetActiveSessionsForAsset(assetID int) int {
	count := 0
	for _, session := range webProxySessions {
		if session.AssetId == assetID {
			count++
		}
	}
	return count
}

// GetAllSessions returns all active sessions
func GetAllSessions() map[string]*WebProxySession {
	return webProxySessions
}

// CountActiveSessions returns the total number of active sessions
func CountActiveSessions() int {
	return len(webProxySessions)
}

// CloseWebSession closes and removes a session
func CloseWebSession(sessionID string) {
	delete(webProxySessions, sessionID)
}
