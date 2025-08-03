package web_proxy

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

// Global session storage
var webProxySessions = make(map[string]*WebProxySession)

// WebProxySession represents an active web proxy session
type WebProxySession struct {
	SessionId     string
	AssetId       int
	AccountId     int
	Asset         *model.Asset
	CreatedAt     time.Time
	LastActivity  time.Time
	LastHeartbeat time.Time // Track heartbeat separately
	IsActive      bool      // Active for concurrent control (heartbeat-based)
	CurrentHost   string
	Permissions   *model.AuthPermissions // User permissions for this asset
	WebConfig     *model.WebConfig       // Web-specific configuration
}

// cleanupExpiredSessions implements layered timeout mechanism
func cleanupExpiredSessions(maxInactiveTime time.Duration) {
	now := time.Now()
	deactivatedCount := 0
	cleanedCount := 0

	// Layer 1: Concurrent control timeout (fast)
	heartbeatTimeout := 45 * time.Second

	// Layer 2: Session expiry timeout (slow, system config)

	for sessionID, session := range webProxySessions {
		// Layer 1: Check heartbeat for concurrent control
		if session.IsActive && !session.LastHeartbeat.IsZero() &&
			now.Sub(session.LastHeartbeat) > heartbeatTimeout {
			// Deactivate session (release concurrent slot AND mark as offline)
			session.IsActive = false
			UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_OFFLINE)
			deactivatedCount++
		}

		// Layer 2: Check session expiry for final cleanup
		shouldDelete := false
		if now.Sub(session.LastActivity) > maxInactiveTime {
			shouldDelete = true
		}

		if shouldDelete {
			// No need to update status again - already done in Layer 1
			delete(webProxySessions, sessionID)
			cleanedCount++
		}
	}

	if deactivatedCount > 0 || cleanedCount > 0 {
		logger.L().Debug("Session cleanup completed",
			zap.Int("deactivated", deactivatedCount),
			zap.Int("deleted", cleanedCount))
	}
}

// StartSessionCleanupRoutine starts background cleanup routine for web sessions
func StartSessionCleanupRoutine() {
	// More frequent cleanup - every 30 seconds to catch closed browser tabs quickly
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			// Use system configured timeout (same as other protocols)
			systemTimeout := time.Duration(model.GlobalConfig.Load().Timeout) * time.Second
			cleanupExpiredSessions(systemTimeout)
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

// UpdateSessionHeartbeat updates the last heartbeat time for a session
func UpdateSessionHeartbeat(sessionID string) {
	if session, exists := webProxySessions[sessionID]; exists {
		now := time.Now()
		wasInactive := !session.IsActive

		session.LastHeartbeat = now
		session.IsActive = true // Re-activate session on heartbeat
		// Heartbeat also counts as activity (user is still viewing the page)
		session.LastActivity = now

		// If session was previously inactive, mark it as online again
		if wasInactive {
			UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_ONLINE)
		}
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
	// First cleanup expired sessions to get accurate count
	systemTimeout := time.Duration(model.GlobalConfig.Load().Timeout) * time.Second
	cleanupExpiredSessions(systemTimeout)

	count := 0
	for _, session := range webProxySessions {
		if session.AssetId == assetID && session.IsActive {
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
	if session, exists := webProxySessions[sessionID]; exists {
		logger.L().Info("Closing web session",
			zap.String("sessionID", sessionID),
			zap.Int("assetID", session.AssetId),
			zap.Int("accountID", session.AccountId))

		// Update database session record to offline status
		UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_OFFLINE)

		delete(webProxySessions, sessionID)
	}
}

// UpdateWebSessionStatus updates the session status in database
func UpdateWebSessionStatus(sessionID string, status int) {
	// Use repository to get and update database session status
	repo := repository.NewSessionRepository()
	if dbSession, err := repo.GetSession(context.Background(), sessionID); err == nil && dbSession != nil {
		now := time.Now()
		dbSession.Status = status
		if status == model.SESSIONSTATUS_OFFLINE {
			dbSession.ClosedAt = &now
		}
		dbSession.UpdatedAt = now

		// Save updated session to database
		fullSession := &gsession.Session{Session: dbSession}
		if err := gsession.UpsertSession(fullSession); err != nil {
			logger.L().Error("Failed to update web session status in database",
				zap.String("sessionID", sessionID),
				zap.Int("status", status),
				zap.Error(err))
		}
	}
}
