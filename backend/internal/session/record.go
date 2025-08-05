package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/storage"
)

type Asciinema struct {
	sessionID  string
	buffer     *bytes.Buffer
	ts         time.Time
	useStorage bool
}

func NewAsciinema(id string, w, h int) (ret *Asciinema, err error) {
	ret = &Asciinema{
		sessionID:  id,
		buffer:     bytes.NewBuffer(nil),
		ts:         time.Now(),
		useStorage: storage.DefaultSessionReplayAdapter != nil,
	}

	// Write Asciinema header information
	header := map[string]any{
		"version":   2,
		"width":     w,
		"height":    h,
		"timestamp": ret.ts.Unix(),
		"title":     id,
		"env": map[string]any{
			"SHELL": "/bin/bash",
			"TERM":  "xterm-256color",
		},
	}

	bs, err := json.Marshal(header)
	if err != nil {
		logger.L().Error("failed to marshal asciinema header", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	ret.buffer.Write(append(bs, '\r', '\n'))

	return ret, nil
}

func (a *Asciinema) Write(p []byte) {
	o := [3]any{}
	o[0] = float64(time.Now().UnixMicro()-a.ts.UnixMicro()) / 1_000_000
	o[1] = "o"
	o[2] = string(p)
	bs, _ := json.Marshal(o)
	a.buffer.Write(append(bs, '\r', '\n'))
}

func (a *Asciinema) Resize(w, h int) {
	r := [3]any{}
	r[0] = float64(time.Now().UnixMicro()-a.ts.UnixMicro()) / 1_000_000
	r[1] = "r"
	r[2] = fmt.Sprintf("%dx%d", w, h)
	bs, _ := json.Marshal(r)
	a.buffer.Write(append(bs, '\r', '\n'))
}

// Close closes the recording and saves to storage
func (a *Asciinema) Close() error {
	if a.useStorage && storage.DefaultSessionReplayAdapter != nil {
		reader := bytes.NewReader(a.buffer.Bytes())
		size := int64(a.buffer.Len())
		err := storage.DefaultSessionReplayAdapter.SaveReplay(a.sessionID, reader, size)
		if err != nil {
			logger.L().Error("Failed to save replay to storage", zap.String("session_id", a.sessionID), zap.Error(err))
			return a.saveToLocalFile()
		}
		return nil
	}
	return a.saveToLocalFile()
}

// saveToLocalFile saves to local filesystem (fallback solution)
func (a *Asciinema) saveToLocalFile() error {
	logger.L().Info("saveToLocalFile called", zap.String("session_id", a.sessionID))

	// Use date hierarchy strategy for local files - directly under base_path
	dateDir := a.ts.Format("2006-01-02")
	replayDir := filepath.Join(config.Cfg.Session.ReplayDir, dateDir)

	if err := os.MkdirAll(replayDir, 0755); err != nil {
		logger.L().Error("create replay directory failed", zap.String("dir", replayDir), zap.Error(err))
		return err
	}

	filePath := filepath.Join(replayDir, fmt.Sprintf("%s.cast", a.sessionID))
	file, err := os.Create(filePath)
	if err != nil {
		logger.L().Error("create replay file failed", zap.String("path", filePath), zap.Error(err))
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, bytes.NewReader(a.buffer.Bytes()))
	if err != nil {
		logger.L().Error("write replay file failed", zap.String("path", filePath), zap.Error(err))
		return err
	}

	logger.L().Info("Replay saved to local file",
		zap.String("session_id", a.sessionID),
		zap.String("path", filePath))

	return nil
}

// GetReplay gets replay file
func GetReplay(sessionID string) (io.ReadCloser, error) {
	// Try to get from storage first
	if storage.DefaultSessionReplayAdapter != nil {
		reader, err := storage.DefaultSessionReplayAdapter.GetReplay(sessionID)
		if err == nil {
			return reader, nil
		}

		logger.L().Warn("Failed to get replay from storage, trying local file",
			zap.String("session_id", sessionID),
			zap.Error(err))
	}

	// Fallback to local file with date hierarchy search
	replayDir := config.Cfg.Session.ReplayDir

	// First try exact path for backward compatibility
	oldFilePath := filepath.Join(replayDir, fmt.Sprintf("%s.cast", sessionID))
	if file, err := os.Open(oldFilePath); err == nil {
		return file, nil
	}

	// Try RDP format (no extension) - guacd saves RDP recordings without .cast extension
	rdpFilePath := filepath.Join(replayDir, sessionID)
	if file, err := os.Open(rdpFilePath); err == nil {
		return file, nil
	}

	// Search in date hierarchy directories (directly under base_path)
	entries, err := os.ReadDir(replayDir)
	if err != nil {
		return nil, fmt.Errorf("replay not found for session %s: %w", sessionID, err)
	}

	// Search in date directories (newest first)
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry.IsDir() {
			// Check if directory name looks like a date (YYYY-MM-DD)
			if len(entry.Name()) == 10 && entry.Name()[4] == '-' && entry.Name()[7] == '-' {
				// Try SSH format (.cast extension)
				filePath := filepath.Join(replayDir, entry.Name(), fmt.Sprintf("%s.cast", sessionID))
				if file, err := os.Open(filePath); err == nil {
					return file, nil
				}

				// Try RDP format (no extension) in date directories
				rdpFilePath := filepath.Join(replayDir, entry.Name(), sessionID)
				if file, err := os.Open(rdpFilePath); err == nil {
					return file, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("replay not found for session %s", sessionID)
}

// ReplayExists checks if replay exists
func ReplayExists(sessionID string) bool {
	// Check storage
	if storage.DefaultSessionReplayAdapter != nil {
		exists, err := storage.DefaultSessionReplayAdapter.ReplayExists(sessionID)
		if err == nil && exists {
			return true
		}
	}

	// Check local file
	replayDir := config.Cfg.Session.ReplayDir
	filePath := filepath.Join(replayDir, fmt.Sprintf("%s.cast", sessionID))
	_, err := os.Stat(filePath)
	return err == nil
}

// DeleteReplay deletes replay
func DeleteReplay(sessionID string) error {
	var lastErr error

	// Delete from storage
	if storage.DefaultSessionReplayAdapter != nil {
		err := storage.DefaultSessionReplayAdapter.DeleteReplay(sessionID)
		if err != nil {
			logger.L().Warn("Failed to delete replay from storage",
				zap.String("session_id", sessionID),
				zap.Error(err))
			lastErr = err
		}
	}

	// Delete from local
	replayDir := config.Cfg.Session.ReplayDir
	filePath := filepath.Join(replayDir, fmt.Sprintf("%s.cast", sessionID))

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		logger.L().Warn("Failed to delete local replay file",
			zap.String("session_id", sessionID),
			zap.String("path", filePath),
			zap.Error(err))
		if lastErr == nil {
			lastErr = err
		}
	}

	return lastErr
}

// MigrateLocalReplaysToStorage migrates local replays to new storage
func MigrateLocalReplaysToStorage() error {
	if storage.DefaultSessionReplayAdapter == nil {
		return fmt.Errorf("storage adapter not initialized")
	}

	replayDir := config.Cfg.Session.ReplayDir
	if _, err := os.Stat(replayDir); os.IsNotExist(err) {
		logger.L().Info("No local replay directory found, skipping migration")
		return nil
	}

	files, err := os.ReadDir(replayDir)
	if err != nil {
		return fmt.Errorf("failed to read replay directory: %w", err)
	}

	var migratedCount, failedCount int

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".cast") {
			continue
		}

		sessionID := strings.TrimSuffix(file.Name(), ".cast")
		filePath := filepath.Join(replayDir, file.Name())

		// Check if already exists in storage
		exists, err := storage.DefaultSessionReplayAdapter.ReplayExists(sessionID)
		if err != nil {
			logger.L().Error("Failed to check replay existence",
				zap.String("session_id", sessionID),
				zap.Error(err))
			continue
		}

		if exists {
			logger.L().Info("Replay already exists in storage, skipping",
				zap.String("session_id", sessionID))
			continue
		}

		// Migrate file
		localFile, err := os.Open(filePath)
		if err != nil {
			logger.L().Error("Failed to open local replay file",
				zap.String("path", filePath),
				zap.Error(err))
			failedCount++
			continue
		}

		info, err := localFile.Stat()
		if err != nil {
			localFile.Close()
			logger.L().Error("Failed to get file info",
				zap.String("path", filePath),
				zap.Error(err))
			failedCount++
			continue
		}

		err = storage.DefaultSessionReplayAdapter.SaveReplay(sessionID, localFile, info.Size())
		localFile.Close()

		if err != nil {
			logger.L().Error("Failed to migrate replay to storage",
				zap.String("session_id", sessionID),
				zap.Error(err))
			failedCount++
			continue
		}

		migratedCount++
		logger.L().Info("Replay migrated successfully",
			zap.String("session_id", sessionID))

		// Optional: Delete local file after successful migration
		// os.Remove(filePath)
	}

	logger.L().Info("Replay migration completed",
		zap.Int("migrated", migratedCount),
		zap.Int("failed", failedCount))

	if failedCount > 0 {
		return fmt.Errorf("migration completed with %d failures", failedCount)
	}

	return nil
}
