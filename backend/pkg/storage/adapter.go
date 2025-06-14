package storage

import (
	"context"
	"io"
	"time"

	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
)

// AdvancedProvider extends Provider with path strategy support
type AdvancedProvider interface {
	Provider
	GetPathStrategy() PathStrategy
}

// SessionReplayAdapter provides session replay storage operations
type SessionReplayAdapter struct {
	provider Provider
}

// NewSessionReplayAdapter creates a new session replay adapter
func NewSessionReplayAdapter(provider Provider) *SessionReplayAdapter {
	return &SessionReplayAdapter{
		provider: provider,
	}
}

// SaveReplay saves a session replay with timestamp-based path generation
func (a *SessionReplayAdapter) SaveReplay(sessionID string, reader io.Reader, size int64) error {
	if a.provider == nil {
		logger.L().Warn("SessionReplayAdapter provider is nil", zap.String("session_id", sessionID))
		return nil // No storage provider available
	}

	ctx := context.Background()

	// Generate key with current timestamp for date-based organization
	key := a.generateReplayKey(sessionID, time.Now())

	return a.provider.Upload(ctx, key, reader, size)
}

// GetReplay retrieves a session replay
func (a *SessionReplayAdapter) GetReplay(sessionID string) (io.ReadCloser, error) {
	if a.provider == nil {
		return nil, nil // No storage provider available
	}

	ctx := context.Background()

	// Try with current date first (most recent)
	key := a.generateReplayKey(sessionID, time.Now())
	reader, err := a.provider.Download(ctx, key)
	if err == nil {
		return reader, nil
	}

	// Fallback to old format for backward compatibility
	oldKey := sessionID + ".cast"
	return a.provider.Download(ctx, oldKey)
}

// DeleteReplay deletes a session replay
func (a *SessionReplayAdapter) DeleteReplay(sessionID string) error {
	if a.provider == nil {
		return nil // No storage provider available
	}

	ctx := context.Background()

	// Try to delete with current date first
	key := a.generateReplayKey(sessionID, time.Now())
	err := a.provider.Delete(ctx, key)
	if err == nil {
		return nil
	}

	// Fallback to old format for backward compatibility
	oldKey := sessionID + ".cast"
	return a.provider.Delete(ctx, oldKey)
}

// ReplayExists checks if a replay exists
func (a *SessionReplayAdapter) ReplayExists(sessionID string) (bool, error) {
	if a.provider == nil {
		return false, nil // No storage provider available
	}

	ctx := context.Background()

	// Try with current date first
	key := a.generateReplayKey(sessionID, time.Now())
	exists, err := a.provider.Exists(ctx, key)
	if err == nil && exists {
		return true, nil
	}

	// Fallback to old format for backward compatibility
	oldKey := sessionID + ".cast"
	return a.provider.Exists(ctx, oldKey)
}

// generateReplayKey generates storage key for replay files
// For date hierarchy strategy: YYYY-MM-DD/sessionID.cast
// For flat strategy: sessionID.cast
func (a *SessionReplayAdapter) generateReplayKey(sessionID string, timestamp time.Time) string {
	// Check if provider supports advanced path generation
	if advProvider, ok := a.provider.(AdvancedProvider); ok {
		strategy := advProvider.GetPathStrategy()
		if strategy == DateHierarchyStrategy {
			// Use date-based path: YYYY-MM-DD/sessionID.cast
			dateDir := timestamp.Format("2006-01-02")
			return dateDir + "/" + sessionID + ".cast"
		}
	}

	// Fallback to flat structure
	return sessionID + ".cast"
}

// SaveReplayWithTimestamp saves a session replay with explicit timestamp
func (a *SessionReplayAdapter) SaveReplayWithTimestamp(sessionID string, reader io.Reader, size int64, timestamp time.Time) error {
	if a.provider == nil {
		logger.L().Warn("SessionReplayAdapter provider is nil", zap.String("session_id", sessionID))
		return nil
	}

	ctx := context.Background()
	key := a.generateReplayKey(sessionID, timestamp)

	return a.provider.Upload(ctx, key, reader, size)
}

// Global adapter instance
var DefaultSessionReplayAdapter *SessionReplayAdapter

// InitializeAdapter initializes the global session replay adapter
func InitializeAdapter(provider Provider) {
	DefaultSessionReplayAdapter = NewSessionReplayAdapter(provider)
	logger.L().Info("Session replay adapter initialized",
		zap.String("provider_type", provider.Type()))
}
