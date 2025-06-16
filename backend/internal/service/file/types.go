package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	// Legacy asset-based file manager
	fm = &FileManager{
		sftps:    map[string]*sftp.Client{},
		lastTime: map[string]time.Time{},
		mtx:      sync.Mutex{},
	}

	// New session-based file manager
	sessionFM = &SessionFileManager{
		sessionSFTP: make(map[string]*sftp.Client),
		sessionSSH:  make(map[string]*ssh.Client),
		lastActive:  make(map[string]time.Time),
	}

	// Session file transfer manager
	sessionTransferManager = &SessionFileTransferManager{
		transfers: make(map[string]*SessionFileTransfer),
	}

	// Progress tracking state
	fileTransfers = make(map[string]*FileTransferProgress)
	transferMutex sync.RWMutex
)

// Session-based file operation errors
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionClosed    = errors.New("session has been closed")
	ErrSessionInactive  = errors.New("session is inactive")
	ErrSFTPNotAvailable = errors.New("SFTP not available for this session")
)

// Global getter functions
func GetFileManager() *FileManager {
	return fm
}

func GetSessionFileManager() *SessionFileManager {
	return sessionFM
}

func GetSessionTransferManager() *SessionFileTransferManager {
	return sessionTransferManager
}

// FileInfo represents file information
type FileInfo struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsLink  bool   `json:"is_link"`
	Target  string `json:"target"`
	ModTime string `json:"mod_time"`
}

// FileManager manages SFTP connections (legacy asset-based)
type FileManager struct {
	sftps    map[string]*sftp.Client
	lastTime map[string]time.Time
	mtx      sync.Mutex
}

func (fm *FileManager) GetFileClient(assetId, accountId int) (cli *sftp.Client, err error) {
	fm.mtx.Lock()
	defer fm.mtx.Unlock()

	key := fmt.Sprintf("%d-%d", assetId, accountId)
	defer func() {
		fm.lastTime[key] = time.Now()
	}()

	cli, ok := fm.sftps[key]
	if ok {
		return
	}

	asset, account, gateway, err := repository.GetAAG(assetId, accountId)
	if err != nil {
		return
	}

	ip, port, err := tunneling.Proxy(false, uuid.New().String(), "sftp,ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := repository.GetAuth(account)
	if err != nil {
		return
	}

	sshCli, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
		User:            account.Account,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		return
	}

	// Create optimized SFTP client
	cli, err = sftp.NewClient(sshCli,
		sftp.MaxPacket(32768),                 // 32KB packets for maximum compatibility
		sftp.MaxConcurrentRequestsPerFile(16), // Increase concurrent requests
		sftp.UseConcurrentWrites(true),        // Enable concurrent writes
		sftp.UseConcurrentReads(true),         // Enable concurrent reads
		sftp.UseFstat(false),                  // Disable fstat for better compatibility
	)
	if err != nil {
		sshCli.Close()
		return
	}
	fm.sftps[key] = cli

	return
}

// SessionFileManager manages SFTP connections per session
type SessionFileManager struct {
	sessionSFTP map[string]*sftp.Client // sessionId -> SFTP client
	sessionSSH  map[string]*ssh.Client  // sessionId -> SSH client
	lastActive  map[string]time.Time    // sessionId -> last active time
	mutex       sync.RWMutex
}

func (sfm *SessionFileManager) InitSessionSFTP(sessionId string, assetId, accountId int) error {
	sfm.mutex.Lock()
	defer sfm.mutex.Unlock()

	// Check if already exists
	if _, exists := sfm.sessionSFTP[sessionId]; exists {
		sfm.lastActive[sessionId] = time.Now()
		return nil
	}

	// CRITICAL OPTIMIZATION: Try to reuse existing SSH connection from terminal session
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	var sshClient *ssh.Client
	var shouldCloseClient = false

	if onlineSession != nil && onlineSession.HasSSHClient() {
		sshClient = onlineSession.GetSSHClient()
		logger.L().Info("REUSING existing SSH connection from terminal session",
			zap.String("sessionId", sessionId))
	} else {
		// Fallback: Create new SSH connection if no existing connection found
		asset, account, gateway, err := repository.GetAAG(assetId, accountId)
		if err != nil {
			return err
		}

		// Use sessionId as proxy identifier for connection reuse
		ip, port, err := tunneling.Proxy(false, sessionId, "sftp,ssh", asset, gateway)
		if err != nil {
			return err
		}

		auth, err := repository.GetAuth(account)
		if err != nil {
			return err
		}

		sshClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
			User:            account.Account,
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("failed to connect SSH for session %s: %w", sessionId, err)
		}
		shouldCloseClient = true // We created it, so we manage its lifecycle
		logger.L().Info("Created new SSH connection for file transfer",
			zap.String("sessionId", sessionId))
	}

	// Create SFTP client with optimized settings for better performance
	sftpClient, err := sftp.NewClient(sshClient,
		sftp.MaxPacket(32768),                 // 32KB packets for maximum compatibility
		sftp.MaxConcurrentRequestsPerFile(16), // Increase concurrent requests per file (default is 3)
		sftp.UseConcurrentWrites(true),        // Enable concurrent writes
		sftp.UseConcurrentReads(true),         // Enable concurrent reads
		sftp.UseFstat(false),                  // Disable fstat for better compatibility
	)
	if err != nil {
		if shouldCloseClient {
			sshClient.Close()
		}
		return fmt.Errorf("failed to create SFTP client for session %s: %w", sessionId, err)
	}

	// Store clients (only store SSH client if we created it)
	sfm.sessionSFTP[sessionId] = sftpClient
	if shouldCloseClient {
		sfm.sessionSSH[sessionId] = sshClient
	}
	sfm.lastActive[sessionId] = time.Now()

	logger.L().Info("SFTP connection initialized for session",
		zap.String("sessionId", sessionId),
		zap.Bool("reusedConnection", !shouldCloseClient))
	return nil
}

func (sfm *SessionFileManager) GetSessionSFTP(sessionId string) (*sftp.Client, error) {
	sfm.mutex.RLock()
	defer sfm.mutex.RUnlock()

	client, exists := sfm.sessionSFTP[sessionId]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Update last active time
	sfm.lastActive[sessionId] = time.Now()
	return client, nil
}

func (sfm *SessionFileManager) CloseSessionSFTP(sessionId string) {
	sfm.mutex.Lock()
	defer sfm.mutex.Unlock()

	if sftpClient, exists := sfm.sessionSFTP[sessionId]; exists {
		sftpClient.Close()
		delete(sfm.sessionSFTP, sessionId)
	}

	// Only close SSH client if we created it (not reused from terminal session)
	if sshClient, exists := sfm.sessionSSH[sessionId]; exists {
		sshClient.Close()
		delete(sfm.sessionSSH, sessionId)
		logger.L().Info("SFTP SSH connection closed for session", zap.String("sessionId", sessionId))
	} else {
		logger.L().Info("SFTP connection closed for session (SSH connection reused)", zap.String("sessionId", sessionId))
	}

	delete(sfm.lastActive, sessionId)
}

func (sfm *SessionFileManager) IsSessionActive(sessionId string) bool {
	sfm.mutex.RLock()
	defer sfm.mutex.RUnlock()

	_, exists := sfm.sessionSFTP[sessionId]
	return exists
}

func (sfm *SessionFileManager) CleanupInactiveSessions(timeout time.Duration) {
	sfm.mutex.Lock()
	defer sfm.mutex.Unlock()

	cutoff := time.Now().Add(-timeout)
	for sessionId, lastActive := range sfm.lastActive {
		if lastActive.Before(cutoff) {
			// Close and remove inactive session
			if sftpClient, exists := sfm.sessionSFTP[sessionId]; exists {
				sftpClient.Close()
				delete(sfm.sessionSFTP, sessionId)
			}

			if sshClient, exists := sfm.sessionSSH[sessionId]; exists {
				sshClient.Close()
				delete(sfm.sessionSSH, sessionId)
			}

			delete(sfm.lastActive, sessionId)
			logger.L().Info("Cleaned up inactive SFTP session",
				zap.String("sessionId", sessionId),
				zap.Duration("inactiveFor", time.Since(lastActive)))
		}
	}
}

func (sfm *SessionFileManager) GetActiveSessionCount() int {
	sfm.mutex.RLock()
	defer sfm.mutex.RUnlock()
	return len(sfm.sessionSFTP)
}

// IFileService defines the file service interface
type IFileService interface {
	// Legacy asset-based operations (for backward compatibility)
	ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error)
	MkdirAll(ctx context.Context, assetId, accountId int, dir string) error
	Create(ctx context.Context, assetId, accountId int, path string) (io.WriteCloser, error)
	Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error)
	Stat(ctx context.Context, assetId, accountId int, path string) (fs.FileInfo, error)
	DownloadMultiple(ctx context.Context, assetId, accountId int, dir string, filenames []string) (io.ReadCloser, string, int64, error)

	// Session-based operations (NEW - high performance)
	SessionLS(ctx context.Context, sessionId, dir string) ([]FileInfo, error)
	SessionMkdir(ctx context.Context, sessionId, dir string) error
	SessionUpload(ctx context.Context, sessionId, targetPath string, file io.Reader, filename string, size int64) error
	SessionUploadWithID(ctx context.Context, transferID, sessionId, targetPath string, file io.Reader, filename string, size int64) error
	SessionDownload(ctx context.Context, sessionId, filePath string) (io.ReadCloser, int64, error)
	SessionDownloadMultiple(ctx context.Context, sessionId, dir string, filenames []string) (io.ReadCloser, string, int64, error)

	// Session lifecycle management
	InitSessionFileClient(sessionId string, assetId, accountId int) error
	CloseSessionFileClient(sessionId string)
	IsSessionActive(sessionId string) bool

	// Progress tracking for session transfers
	GetSessionTransferProgress(ctx context.Context, sessionId string) ([]*SessionFileTransferProgress, error)
	GetTransferProgress(ctx context.Context, transferId string) (*SessionFileTransferProgress, error)

	// File history and other operations
	AddFileHistory(ctx context.Context, history *model.FileHistory) error
	BuildFileHistoryQuery(ctx *gin.Context) *gorm.DB
	RecordFileHistory(ctx context.Context, operation, dir, filename string, assetId, accountId int, sessionId ...string) error
	RecordFileHistoryBySession(ctx context.Context, sessionId, operation, path string) error

	// Utility methods
	GetActionCode(operation string) int
	ValidateAndNormalizePath(basePath, userPath string) (string, error)
	GetRDPDrivePath(assetId int) (string, error)
}

// FileService implements IFileService
type FileService struct {
	repo repository.IFileRepository
}

// RDP File related structures
type RDPFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

type RDPMkdirRequest struct {
	Path string `json:"path" binding:"required"`
}

type RDPProgressWriter struct {
	writer     io.Writer
	transfer   *guacd.FileTransfer
	transferId string
	written    int64
}

// Session transfer related structures
type SessionFileTransfer struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Offset    int64     `json:"offset"`
	Status    string    `json:"status"` // "pending", "uploading", "completed", "failed"
	IsUpload  bool      `json:"is_upload"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Error     string    `json:"error,omitempty"`
	mutex     sync.Mutex
}

type SessionFileTransferProgress struct {
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

type SessionFileTransferManager struct {
	transfers map[string]*SessionFileTransfer
	mutex     sync.RWMutex
}

func (m *SessionFileTransferManager) CreateTransfer(sessionID, filename string, size int64, isUpload bool) *SessionFileTransfer {
	return m.CreateTransferWithID(generateTransferID(), sessionID, filename, size, isUpload)
}

func (m *SessionFileTransferManager) CreateTransferWithID(transferID, sessionID, filename string, size int64, isUpload bool) *SessionFileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	transfer := &SessionFileTransfer{
		ID:        transferID,
		SessionID: sessionID,
		Filename:  filename,
		Size:      size,
		Offset:    0,
		Status:    "pending",
		IsUpload:  isUpload,
		Created:   time.Now(),
		Updated:   time.Now(),
	}

	m.transfers[transferID] = transfer
	return transfer
}

func (m *SessionFileTransferManager) GetTransfer(id string) *SessionFileTransfer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.transfers[id]
}

func (m *SessionFileTransferManager) GetTransfersBySession(sessionID string) []*SessionFileTransfer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var transfers []*SessionFileTransfer
	for _, transfer := range m.transfers {
		if transfer.SessionID == sessionID {
			transfers = append(transfers, transfer)
		}
	}
	return transfers
}

func (m *SessionFileTransferManager) GetSessionProgress(sessionID string) []*SessionFileTransferProgress {
	transfers := m.GetTransfersBySession(sessionID)
	var progressList []*SessionFileTransferProgress
	for _, transfer := range transfers {
		progressList = append(progressList, transfer.GetProgress())
	}
	return progressList
}

func (m *SessionFileTransferManager) RemoveTransfer(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.transfers, id)
}

func (m *SessionFileTransferManager) CleanupCompletedTransfers(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, transfer := range m.transfers {
		if (transfer.Status == "completed" || transfer.Status == "failed") && transfer.Updated.Before(cutoff) {
			delete(m.transfers, id)
		}
	}
}

// Progress tracking structures
type FileTransferProgress struct {
	TotalSize       int64
	TransferredSize int64
	Status          string // "transferring", "completed", "failed"
	Type            string // "sftp", "rdp"
}

type ProgressWriter struct {
	writer   io.Writer
	transfer *SessionFileTransfer
	written  int64
}

func NewProgressWriter(writer io.Writer, transfer *SessionFileTransfer) *ProgressWriter {
	return &ProgressWriter{
		writer:   writer,
		transfer: transfer,
		written:  0,
	}
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.written += int64(n)
		pw.transfer.UpdateProgress(pw.written)
	}

	if err != nil {
		pw.transfer.SetError(err)
	}

	return n, err
}

type FileProgressWriter struct {
	writer       io.Writer
	transferId   string
	written      int64
	lastUpdate   time.Time
	updateBytes  int64 // Bytes written since last progress update
	updateTicker int64 // Simple counter to reduce time.Now() calls
}

func NewFileProgressWriter(writer io.Writer, transferId string) *FileProgressWriter {
	return &FileProgressWriter{
		writer:     writer,
		transferId: transferId,
		written:    0,
		lastUpdate: time.Now(),
	}
}

func (pw *FileProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.written += int64(n)
		pw.updateBytes += int64(n)
		pw.updateTicker++

		// Update progress every 256KB or every 500 writes to reduce memory overhead
		// Larger intervals reduce GC pressure and memory fragmentation
		if pw.updateBytes >= 262144 || pw.updateTicker%500 == 0 {
			UpdateTransferProgress(pw.transferId, 0, pw.written, "transferring")
			pw.updateBytes = 0
			pw.lastUpdate = time.Now()
		}
	}

	if err != nil {
		UpdateTransferProgress(pw.transferId, 0, -1, "failed")
	}

	return n, err
}

// SessionFileTransfer methods
func (t *SessionFileTransfer) UpdateProgress(offset int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Offset = offset
	t.Updated = time.Now()

	if t.Status == "pending" {
		t.Status = "uploading"
	}

	if t.Size > 0 && t.Offset >= t.Size {
		t.Status = "completed"
	}
}

func (t *SessionFileTransfer) SetError(err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Error = err.Error()
	t.Status = "failed"
	t.Updated = time.Now()
}

// GetProgress returns the current progress information
func (t *SessionFileTransfer) GetProgress() *SessionFileTransferProgress {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var percentage float64
	if t.Size > 0 {
		percentage = float64(t.Offset) / float64(t.Size) * 100
	}

	// Calculate speed and ETA
	var speed int64
	var eta int64
	if !t.Created.Equal(t.Updated) && t.Offset > 0 {
		duration := t.Updated.Sub(t.Created).Seconds()
		speed = int64(float64(t.Offset) / duration)

		if speed > 0 && t.Size > t.Offset {
			eta = (t.Size - t.Offset) / speed
		}
	}

	return &SessionFileTransferProgress{
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

// Progress tracking functions
func CreateTransferProgress(transferId, transferType string) {
	transferMutex.Lock()
	fileTransfers[transferId] = &FileTransferProgress{
		TotalSize:       0,
		TransferredSize: 0,
		Status:          "transferring",
		Type:            transferType,
	}
	transferMutex.Unlock()
}

func UpdateTransferProgress(transferId string, totalSize, transferredSize int64, status string) {
	transferMutex.Lock()
	if progress, exists := fileTransfers[transferId]; exists {
		if totalSize > 0 {
			progress.TotalSize = totalSize
		}
		if transferredSize >= 0 {
			progress.TransferredSize = transferredSize
		}
		if status != "" {
			progress.Status = status
		}
	}
	transferMutex.Unlock()
}

func CleanupTransferProgress(transferId string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		transferMutex.Lock()
		delete(fileTransfers, transferId)
		transferMutex.Unlock()
	}()
}

func GetTransferProgressById(transferId string) (*FileTransferProgress, bool) {
	transferMutex.RLock()
	progress, exists := fileTransfers[transferId]
	transferMutex.RUnlock()
	return progress, exists
}

// Utility functions
func generateTransferID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func sanitizeFilename(filename string) (string, error) {
	// Remove any directory separators
	cleaned := filepath.Base(filename)

	// Check for dangerous patterns
	if strings.Contains(cleaned, "..") || cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("invalid filename")
	}

	// Additional security checks
	if strings.HasPrefix(cleaned, ".") && len(cleaned) > 1 {
		// Allow hidden files but validate they're not dangerous
	}

	return cleaned, nil
}

func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	permissionKeywords := []string{
		"permission denied",
		"access denied",
		"unauthorized",
		"forbidden",
		"not authorized",
		"insufficient privileges",
		"operation not permitted",
		"sftp: permission denied",
		"ssh: permission denied",
	}

	for _, keyword := range permissionKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}
