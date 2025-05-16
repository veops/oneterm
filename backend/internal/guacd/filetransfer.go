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
	ID        string
	Filename  string
	Path      string
	Size      int64
	Offset    int64
	Created   time.Time
	Completed bool
	IsUpload  bool
	file      *os.File
	mutex     sync.Mutex
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
func (m *FileTransferManager) CreateUpload(filename, drivePath string) (*FileTransfer, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := uuid.New().String()
	fullPath := filepath.Join(drivePath, filename)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	transfer := &FileTransfer{
		ID:       id,
		Filename: filename,
		Path:     fullPath,
		Created:  time.Now(),
		IsUpload: true,
		file:     file,
	}

	m.transfers[id] = transfer
	logger.L().Debug("Created file upload", zap.String("id", id), zap.String("filename", filename))
	return transfer, nil
}

// CreateDownload creates a download file transfer
func (m *FileTransferManager) CreateDownload(filename, drivePath string) (*FileTransfer, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	id := uuid.New().String()
	fullPath := filepath.Join(drivePath, filename)

	// Open file
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	transfer := &FileTransfer{
		ID:       id,
		Filename: filename,
		Path:     fullPath,
		Size:     stat.Size(),
		Created:  time.Now(),
		IsUpload: false,
		file:     file,
	}

	m.transfers[id] = transfer
	logger.L().Debug("Created file download", zap.String("id", id), zap.String("filename", filename))
	return transfer, nil
}

// GetTransfer gets a transfer by ID
func (m *FileTransferManager) GetTransfer(id string) *FileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.transfers[id]
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

	n, err := t.file.Write(data)
	if err != nil {
		return n, err
	}

	t.Offset += int64(n)
	return n, nil
}

// Read reads data from a download file
func (t *FileTransfer) Read(p []byte) (int, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.IsUpload {
		return 0, fmt.Errorf("cannot read from upload transfer")
	}

	n, err := t.file.Read(p)
	if err != nil {
		if err == io.EOF {
			t.Completed = true
		}
		return n, err
	}

	t.Offset += int64(n)
	if t.Offset >= t.Size {
		t.Completed = true
	}

	return n, nil
}

// Close closes the file transfer
func (t *FileTransfer) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Completed = true
	if t.file != nil {
		return t.file.Close()
	}
	return nil
}
