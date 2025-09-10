package web_proxy

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

var webProxySessions = make(map[string]*WebProxySession)

var (
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
	cleanupWg     sync.WaitGroup
)

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
	SessionPerms  *SessionPermissions // Cached session permissions for proxy phase
	WebConfig     *model.WebConfig
}

func cleanupExpiredSessions(systemMaxInactiveTime time.Duration) {
	now := time.Now()
	deactivatedCount := 0
	cleanedCount := 0

	heartbeatTimeout := 30 * time.Second

	for sessionID, session := range webProxySessions {
		if session.IsActive && !session.LastHeartbeat.IsZero() &&
			now.Sub(session.LastHeartbeat) > heartbeatTimeout {
			session.IsActive = false
			UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_OFFLINE)
			deactivatedCount++
		}

		effectiveTimeout := systemMaxInactiveTime

		shouldDelete := false
		if now.Sub(session.LastActivity) > effectiveTimeout {
			shouldDelete = true
		}

		if shouldDelete {
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

func StartSessionCleanupRoutine() {
	cleanupCtx, cleanupCancel = context.WithCancel(context.Background())

	ticker := time.NewTicker(30 * time.Second)
	cleanupWg.Add(1)
	go func() {
		defer cleanupWg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-cleanupCtx.Done():
				logger.L().Info("Web proxy session cleanup stopped")
				return
			case <-ticker.C:
				systemTimeout := time.Duration(model.GlobalConfig.Load().Timeout) * time.Second
				cleanupExpiredSessions(systemTimeout)
			}
		}
	}()
}

func StopSessionCleanupRoutine() {
	if cleanupCancel != nil {
		cleanupCancel()
		cleanupWg.Wait()
		logger.L().Info("Web proxy session cleanup routine stopped")
	}
}

func GetSession(sessionID string) (*WebProxySession, bool) {
	session, exists := webProxySessions[sessionID]
	return session, exists
}

func StoreSession(sessionID string, session *WebProxySession) {
	webProxySessions[sessionID] = session
}

func DeleteSession(sessionID string) {
	delete(webProxySessions, sessionID)
}

func UpdateSessionActivity(sessionID string) {
	if session, exists := webProxySessions[sessionID]; exists {
		session.LastActivity = time.Now()
	}
}

func UpdateSessionHeartbeat(sessionID string) {
	if session, exists := webProxySessions[sessionID]; exists {
		now := time.Now()
		wasInactive := !session.IsActive

		session.LastHeartbeat = now
		session.IsActive = true // Re-activate session on heartbeat
		session.LastActivity = now

		if wasInactive {
			UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_ONLINE)
		}
	}
}

func UpdateSessionHost(sessionID string, host string) {
	if session, exists := webProxySessions[sessionID]; exists {
		session.CurrentHost = host
	}
}

func GetActiveSessionsForAsset(assetID int) int {
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

func GetAllSessions() map[string]*WebProxySession {
	return webProxySessions
}

func CountActiveSessions() int {
	return len(webProxySessions)
}

func CloseWebSession(sessionID string) {
	if session, exists := webProxySessions[sessionID]; exists {
		logger.L().Info("Closing web session",
			zap.String("sessionID", sessionID),
			zap.Int("assetID", session.AssetId),
			zap.Int("accountID", session.AccountId))

		UpdateWebSessionStatus(sessionID, model.SESSIONSTATUS_OFFLINE)

		delete(webProxySessions, sessionID)
	}
}

func UpdateWebSessionStatus(sessionID string, status int) {
	repo := repository.NewSessionRepository()
	if dbSession, err := repo.GetSession(context.Background(), sessionID); err == nil && dbSession != nil {
		now := time.Now()
		dbSession.Status = status
		if status == model.SESSIONSTATUS_OFFLINE {
			dbSession.ClosedAt = &now
		}
		dbSession.UpdatedAt = now

		fullSession := &gsession.Session{Session: dbSession}
		if err := gsession.UpsertSession(fullSession); err != nil {
			logger.L().Error("Failed to update web session status in database",
				zap.String("sessionID", sessionID),
				zap.Int("status", status),
				zap.Error(err))
		}
	}
}
