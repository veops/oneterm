package guacd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
)

// FileTransferManager manages RDP file transfers
type FileTransferManager struct {
	transfers map[string]*FileTransfer
	mutex     sync.Mutex
}

// FileTransfer represents a single file transfer
type FileTransfer struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Completed bool      `json:"completed"`
	IsUpload  bool      `json:"is_upload"`
	Status    string    `json:"status"` // "pending", "uploading", "completed", "failed"
	Error     string    `json:"error,omitempty"`
	file      *os.File
	mutex     sync.Mutex
}

// FileTransferProgress represents transfer progress information
type FileTransferProgress struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Filename   string    `json:"filename"`
	Size       int64     `json:"size"`
	Offset     int64     `json:"offset"`
	Percentage float64   `json:"percentage"`
	Status     string    `json:"status"`
	IsUpload   bool      `json:"is_upload"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
	Error      string    `json:"error,omitempty"`
	Speed      int64     `json:"speed"` // bytes per second
	ETA        int64     `json:"eta"`   // estimated time to completion in seconds
}

// Global file transfer manager instance
var (
	DefaultFileTransferManager = NewFileTransferManager()
)

// NewFileTransferManager creates a new file transfer manager
func NewFileTransferManager() *FileTransferManager {
	return &FileTransferManager{
		transfers: make(map[string]*FileTransfer),
	}
}

// CreateUpload creates an upload file transfer
func (m *FileTransferManager) CreateUpload(sessionID, filename, drivePath string) (*FileTransfer, error) {
	return m.CreateUploadWithID("", sessionID, filename, drivePath)
}

// CreateUploadWithID creates an upload file transfer with custom ID
func (m *FileTransferManager) CreateUploadWithID(transferID, sessionID, filename, drivePath string) (*FileTransfer, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var id string
	if transferID != "" {
		if _, exists := m.transfers[transferID]; exists {
			return nil, fmt.Errorf("transfer ID already exists: %s", transferID)
		}
		id = transferID
	} else {
		id = uuid.New().String()
	}

	fullPath := filepath.Join(drivePath, filename)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	now := time.Now()
	transfer := &FileTransfer{
		ID:        id,
		SessionID: sessionID,
		Filename:  filename,
		Path:      fullPath,
		Created:   now,
		Updated:   now,
		IsUpload:  true,
		Status:    "pending",
		file:      file,
	}

	m.transfers[id] = transfer
	logger.L().Debug("Created file upload", zap.String("id", id), zap.String("sessionId", sessionID), zap.String("filename", filename))
	return transfer, nil
}

// CreateDownload creates a download file transfer (no progress tracking)
func (m *FileTransferManager) CreateDownload(sessionID, filename, drivePath string) (*FileTransfer, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := uuid.New().String()
	fullPath := filepath.Join(drivePath, filename)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	now := time.Now()
	transfer := &FileTransfer{
		ID:        id,
		SessionID: sessionID,
		Filename:  filename,
		Path:      fullPath,
		Size:      stat.Size(),
		Created:   now,
		Updated:   now,
		IsUpload:  false,
		Status:    "completed",
		file:      file,
	}

	logger.L().Debug("Created file download", zap.String("id", id), zap.String("sessionId", sessionID), zap.String("filename", filename))
	return transfer, nil
}

// GetTransfer gets a transfer by ID
func (m *FileTransferManager) GetTransfer(id string) *FileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.transfers[id]
}

// GetTransfersBySession gets all transfers for a session
func (m *FileTransferManager) GetTransfersBySession(sessionID string) []*FileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var transfers []*FileTransfer
	for _, transfer := range m.transfers {
		if transfer.SessionID == sessionID {
			transfers = append(transfers, transfer)
		}
	}
	return transfers
}

// GetAllTransfers gets all active transfers
func (m *FileTransferManager) GetAllTransfers() []*FileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var transfers []*FileTransfer
	for _, transfer := range m.transfers {
		transfers = append(transfers, transfer)
	}
	return transfers
}

// GetTransferProgress gets progress information for a transfer
func (m *FileTransferManager) GetTransferProgress(id string) (*FileTransferProgress, error) {
	transfer := m.GetTransfer(id)
	if transfer == nil {
		return nil, fmt.Errorf("transfer not found")
	}

	return transfer.GetProgress(), nil
}

// GetSessionProgress gets progress information for all transfers in a session
func (m *FileTransferManager) GetSessionProgress(sessionID string) ([]*FileTransferProgress, error) {
	transfers := m.GetTransfersBySession(sessionID)

	var progresses []*FileTransferProgress
	for _, transfer := range transfers {
		progresses = append(progresses, transfer.GetProgress())
	}

	return progresses, nil
}

// RemoveTransfer removes a transfer by ID
func (m *FileTransferManager) RemoveTransfer(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if transfer, exists := m.transfers[id]; exists {
		if transfer.file != nil {
			transfer.file.Close()
		}
		delete(m.transfers, id)
	}
}

// CleanupCompletedTransfers removes completed transfers older than specified duration
func (m *FileTransferManager) CleanupCompletedTransfers(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, transfer := range m.transfers {
		if transfer.Completed && transfer.Updated.Before(cutoff) {
			if transfer.file != nil {
				transfer.file.Close()
			}
			delete(m.transfers, id)
			logger.L().Debug("Cleaned up completed transfer", zap.String("id", id), zap.String("filename", transfer.Filename))
		}
	}
}

// Write writes data to an upload file
func (t *FileTransfer) Write(data []byte) (int, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.Completed {
		return 0, fmt.Errorf("transfer already completed")
	}

	if !t.IsUpload {
		return 0, fmt.Errorf("cannot write to download transfer")
	}

	if t.Status == "pending" {
		t.Status = "uploading"
	}

	n, err := t.file.Write(data)
	if err != nil {
		t.Status = "failed"
		t.Error = err.Error()
		t.Updated = time.Now()
		return n, err
	}

	t.Offset += int64(n)
	t.Updated = time.Now()

	if t.Size > 0 && t.Offset >= t.Size {
		t.Completed = true
		t.Status = "completed"
	}

	return n, nil
}

// Read reads data from a download file (no progress tracking)
func (t *FileTransfer) Read(p []byte) (int, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.IsUpload {
		return 0, fmt.Errorf("cannot read from upload transfer")
	}

	n, err := t.file.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}

	return n, err
}

// SetSize sets the total size for the transfer (useful for uploads where size is known)
func (t *FileTransfer) SetSize(size int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.Size = size
}

// GetProgress returns the current progress information
func (t *FileTransfer) GetProgress() *FileTransferProgress {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var percentage float64
	if t.Size > 0 {
		percentage = float64(t.Offset) / float64(t.Size) * 100
	}

	var speed int64
	var eta int64
	if !t.Created.Equal(t.Updated) && t.Offset > 0 {
		duration := t.Updated.Sub(t.Created).Seconds()
		speed = int64(float64(t.Offset) / duration)

		if speed > 0 && t.Size > t.Offset {
			eta = (t.Size - t.Offset) / speed
		}
	}

	return &FileTransferProgress{
		ID:         t.ID,
		SessionID:  t.SessionID,
		Filename:   t.Filename,
		Size:       t.Size,
		Offset:     t.Offset,
		Percentage: percentage,
		Status:     t.Status,
		IsUpload:   t.IsUpload,
		Created:    t.Created,
		Updated:    t.Updated,
		Error:      t.Error,
		Speed:      speed,
		ETA:        eta,
	}
}

// Close closes the file transfer
func (t *FileTransfer) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.Completed {
		if t.Status != "failed" {
			t.Status = "completed"
		}
		t.Completed = true
		t.Updated = time.Now()
	}

	if t.file != nil {
		return t.file.Close()
	}
	return nil
}
