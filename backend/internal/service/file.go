package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	dbpkg "github.com/veops/oneterm/pkg/db"
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

	// Global file service instance
	DefaultFileService IFileService
)

// Session-based file operation errors
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionClosed    = errors.New("session has been closed")
	ErrSessionInactive  = errors.New("session is inactive")
	ErrSFTPNotAvailable = errors.New("SFTP not available for this session")
)

// SessionFileManager manages SFTP connections per session
type SessionFileManager struct {
	sessionSFTP map[string]*sftp.Client // sessionId -> SFTP client
	sessionSSH  map[string]*ssh.Client  // sessionId -> SSH client
	lastActive  map[string]time.Time    // sessionId -> last active time
	mutex       sync.RWMutex
}

func GetSessionFileManager() *SessionFileManager {
	return sessionFM
}

// InitSessionSFTP initializes SFTP connection for a session
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
		asset, account, gateway, err := GetAAG(assetId, accountId)
		if err != nil {
			return err
		}

		// Use sessionId as proxy identifier for connection reuse
		ip, port, err := tunneling.Proxy(false, sessionId, "sftp,ssh", asset, gateway)
		if err != nil {
			return err
		}

		auth, err := GetAuth(account)
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

// GetSessionSFTP gets SFTP client for a session
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

// CloseSessionSFTP closes and removes SFTP connection for a session
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

// IsSessionActive checks if session has active SFTP connection
func (sfm *SessionFileManager) IsSessionActive(sessionId string) bool {
	sfm.mutex.RLock()
	defer sfm.mutex.RUnlock()

	_, exists := sfm.sessionSFTP[sessionId]
	return exists
}

// CleanupInactiveSessions removes SFTP connections for inactive sessions
func (sfm *SessionFileManager) CleanupInactiveSessions(timeout time.Duration) {
	sfm.mutex.Lock()
	defer sfm.mutex.Unlock()

	now := time.Now()
	var toRemove []string

	for sessionId, lastActive := range sfm.lastActive {
		if now.Sub(lastActive) > timeout {
			toRemove = append(toRemove, sessionId)
		}
	}

	for _, sessionId := range toRemove {
		if sftpClient, exists := sfm.sessionSFTP[sessionId]; exists {
			sftpClient.Close()
			delete(sfm.sessionSFTP, sessionId)
		}
		if sshClient, exists := sfm.sessionSSH[sessionId]; exists {
			sshClient.Close()
			delete(sfm.sessionSSH, sessionId)
		}
		delete(sfm.lastActive, sessionId)
		logger.L().Info("Cleaned up inactive session SFTP connection", zap.String("sessionId", sessionId))
	}
}

// GetActiveSessionCount returns number of active session SFTP connections
func (sfm *SessionFileManager) GetActiveSessionCount() int {
	sfm.mutex.RLock()
	defer sfm.mutex.RUnlock()
	return len(sfm.sessionSFTP)
}

// InitFileService initializes the global file service
func InitFileService() {
	repo := repository.NewFileRepository(dbpkg.DB)
	DefaultFileService = NewFileService(repo)
}

func init() {
	// Legacy file manager cleanup
	go func() {
		tk := time.NewTicker(time.Minute)
		for {
			<-tk.C
			func() {
				fm.mtx.Lock()
				defer fm.mtx.Unlock()
				for k, v := range fm.lastTime {
					if v.Before(time.Now().Add(time.Minute * 10)) {
						delete(fm.sftps, k)
						delete(fm.lastTime, k)
					}
				}
			}()
		}
	}()

	// Session-based file manager cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
		defer ticker.Stop()

		for {
			<-ticker.C
			// Clean up sessions inactive for more than 30 minutes
			GetSessionFileManager().CleanupInactiveSessions(30 * time.Minute)
		}
	}()
}

type FileManager struct {
	sftps    map[string]*sftp.Client
	lastTime map[string]time.Time
	mtx      sync.Mutex
}

type FileInfo struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	IsLink  bool   `json:"is_link"`
	Target  string `json:"target"`
	ModTime string `json:"mod_time"`
}

func GetFileManager() *FileManager {
	return fm
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

	asset, account, gateway, err := GetAAG(assetId, accountId)
	if err != nil {
		return
	}

	ip, port, err := tunneling.Proxy(false, uuid.New().String(), "sftp,ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := GetAuth(account)
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

// File service interface
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

// File service implementation
type FileService struct {
	repo repository.IFileRepository
}

func NewFileService(repo repository.IFileRepository) IFileService {
	return &FileService{
		repo: repo,
	}
}

// ReadDir gets directory listing
func (s *FileService) ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.ReadDir(dir)
}

// MkdirAll creates a directory
func (s *FileService) MkdirAll(ctx context.Context, assetId, accountId int, dir string) error {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return err
	}
	return cli.MkdirAll(dir)
}

// Create creates a file
func (s *FileService) Create(ctx context.Context, assetId, accountId int, path string) (io.WriteCloser, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}

	// Check if parent directory exists, create only if not exists
	parentDir := filepath.Dir(path)
	if parentDir != "" && parentDir != "." && parentDir != "/" {
		if _, err := cli.Stat(parentDir); err != nil {
			// Directory doesn't exist, create it
			if err := cli.MkdirAll(parentDir); err != nil {
				return nil, fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
			}
		}
	}

	return cli.Create(path)
}

// Open opens a file
func (s *FileService) Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}

	return cli.Open(path)
}

// Stat gets file/directory information
func (s *FileService) Stat(ctx context.Context, assetId, accountId int, path string) (fs.FileInfo, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}

	return cli.Stat(path)
}

// DownloadMultiple handles downloading single file or multiple files/directories as ZIP
func (s *FileService) DownloadMultiple(ctx context.Context, assetId, accountId int, dir string, filenames []string) (io.ReadCloser, string, int64, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, "", 0, err
	}

	// Validate and sanitize all filenames for security
	var sanitizedFilenames []string
	for _, filename := range filenames {
		sanitized, err := sanitizeFilename(filename)
		if err != nil {
			return nil, "", 0, fmt.Errorf("invalid filename '%s': %v", filename, err)
		}

		sanitizedFilenames = append(sanitizedFilenames, sanitized)
	}

	// If only one file, check if it's a regular file first
	if len(sanitizedFilenames) == 1 {
		fullPath := filepath.Join(dir, sanitizedFilenames[0])
		fileInfo, err := cli.Stat(fullPath)
		if err != nil {
			return nil, "", 0, err
		}

		// If it's a regular file, return directly
		if !fileInfo.IsDir() {
			reader, err := cli.Open(fullPath)
			if err != nil {
				return nil, "", 0, err
			}
			return reader, sanitizedFilenames[0], fileInfo.Size(), nil
		}
	}

	// Multiple files or contains directory, create ZIP
	return s.createZipArchive(cli, dir, sanitizedFilenames)
}

// createZipArchive creates a ZIP archive containing the specified files/directories
func (s *FileService) createZipArchive(cli *sftp.Client, baseDir string, filenames []string) (io.ReadCloser, string, int64, error) {
	// Generate ZIP filename
	var zipName string
	if len(filenames) == 1 {
		zipName = filenames[0] + ".zip"
	} else {
		zipName = "download.zip"
	}

	// Use pipe for true streaming without memory buffering
	pipeReader, pipeWriter := io.Pipe()

	// Create ZIP in a separate goroutine
	go func() {
		defer pipeWriter.Close()

		zipWriter := zip.NewWriter(pipeWriter)
		defer zipWriter.Close()

		// Add each file/directory to ZIP
		for _, filename := range filenames {
			fullPath := filepath.Join(baseDir, filename)

			if err := s.addToZip(cli, zipWriter, baseDir, filename, fullPath); err != nil {
				logger.L().Error("Failed to add file to ZIP", zap.String("path", fullPath), zap.Error(err))
				pipeWriter.CloseWithError(err)
				return
			}
		}
	}()

	// Return pipe reader for streaming, size unknown (-1)
	return pipeReader, zipName, -1, nil
}

// addToZip recursively adds files/directories to ZIP archive
func (s *FileService) addToZip(cli *sftp.Client, zipWriter *zip.Writer, baseDir, relativePath, fullPath string) error {
	fileInfo, err := cli.Stat(fullPath)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		// Add directory
		return s.addDirToZip(cli, zipWriter, fullPath, relativePath)
	} else {
		// Add file
		return s.addFileToZip(cli, zipWriter, fullPath, relativePath)
	}
}

// addFileToZip adds a single file to ZIP archive
func (s *FileService) addFileToZip(cli *sftp.Client, zipWriter *zip.Writer, fullPath, relativePath string) error {
	// Get file info first to preserve metadata
	fileInfo, err := cli.Stat(fullPath)
	if err != nil {
		return err
	}

	// Open remote file
	file, err := cli.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create FileHeader with original file metadata
	header := &zip.FileHeader{
		Name:     relativePath,
		Method:   zip.Deflate,
		Modified: fileInfo.ModTime(), // Preserve original modification time
	}

	// Set file mode
	header.SetMode(fileInfo.Mode())

	// Create file in ZIP with header
	zipFile, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Copy file content
	_, err = io.Copy(zipFile, file)
	return err
}

// addDirToZip recursively adds a directory to ZIP archive
func (s *FileService) addDirToZip(cli *sftp.Client, zipWriter *zip.Writer, fullPath, relativePath string) error {
	// Get directory info to preserve metadata
	dirInfo, err := cli.Stat(fullPath)
	if err != nil {
		return err
	}

	// Read directory contents
	entries, err := cli.ReadDir(fullPath)
	if err != nil {
		return err
	}

	// If directory is empty, create directory entry with preserved timestamp
	if len(entries) == 0 {
		header := &zip.FileHeader{
			Name:     relativePath + "/",
			Method:   zip.Store,         // Directories are not compressed
			Modified: dirInfo.ModTime(), // Preserve original modification time
		}
		header.SetMode(dirInfo.Mode())

		_, err := zipWriter.CreateHeader(header)
		return err
	}

	// Recursively add each entry in the directory
	for _, entry := range entries {
		entryFullPath := filepath.Join(fullPath, entry.Name())
		entryRelativePath := filepath.Join(relativePath, entry.Name())

		if err := s.addToZip(cli, zipWriter, fullPath, entryRelativePath, entryFullPath); err != nil {
			return err
		}
	}

	return nil
}

// AddFileHistory adds a file history record
func (s *FileService) AddFileHistory(ctx context.Context, history *model.FileHistory) error {
	return s.repo.AddFileHistory(ctx, history)
}

// BuildFileHistoryQuery builds query for file history with filters (for doGet pattern)
func (s *FileService) BuildFileHistoryQuery(ctx *gin.Context) *gorm.DB {
	db := dbpkg.DB.Model(&model.FileHistory{})

	db = dbpkg.FilterSearch(ctx, db, "dir", "filename")

	// Apply exact match filters
	db = dbpkg.FilterEqual(ctx, db, "status", "uid", "asset_id", "account_id", "action")

	// Apply client IP filter
	if clientIp := ctx.Query("client_ip"); clientIp != "" {
		db = db.Where("client_ip = ?", clientIp)
	}

	// Apply date range filters
	if start := ctx.Query("start"); start != "" {
		db = db.Where("created_at >= ?", start)
	}

	if end := ctx.Query("end"); end != "" {
		db = db.Where("created_at <= ?", end)
	}

	return db
}

// RDP file transfer methods implementation

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

func NewRDPProgressWriter(writer io.Writer, transfer *guacd.FileTransfer, transferId string) *RDPProgressWriter {
	return &RDPProgressWriter{
		writer:     writer,
		transfer:   transfer,
		transferId: transferId,
		written:    0,
	}
}

func (pw *RDPProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}
	pw.written += int64(n)

	// Update unified progress tracking
	UpdateTransferProgress(pw.transferId, 0, pw.written, "")

	// Also update guacd transfer
	if pw.transfer != nil {
		pw.transfer.Write(p[:n])
	}
	return n, nil
}

// Progress tracking implementation

// GetSessionTransferProgress gets progress information for all transfers in a session
func (s *FileService) GetSessionTransferProgress(ctx context.Context, sessionId string) ([]*SessionFileTransferProgress, error) {
	transferManager := GetSessionTransferManager()
	return transferManager.GetSessionProgress(sessionId), nil
}

// GetTransferProgress gets progress information for a specific transfer
func (s *FileService) GetTransferProgress(ctx context.Context, transferId string) (*SessionFileTransferProgress, error) {
	transferManager := GetSessionTransferManager()
	transfer := transferManager.GetTransfer(transferId)
	if transfer == nil {
		return nil, fmt.Errorf("transfer not found: %s", transferId)
	}
	return transfer.GetProgress(), nil
}

// Session-based file operations (NEW - high performance)

// SessionLS lists directory contents for a session
func (s *FileService) SessionLS(ctx context.Context, sessionId, dir string) ([]FileInfo, error) {
	client, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return nil, err
	}

	fileInfos, err := client.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Convert to our FileInfo structure
	var result []FileInfo
	for _, fi := range fileInfos {
		var target string
		if fi.Mode()&os.ModeSymlink != 0 {
			linkPath := filepath.Join(dir, fi.Name())
			if linkTarget, err := client.ReadLink(linkPath); err == nil {
				target = linkTarget
			}
		}

		result = append(result, FileInfo{
			Name:    fi.Name(),
			IsDir:   fi.IsDir(),
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			IsLink:  fi.Mode()&os.ModeSymlink != 0,
			Target:  target,
			ModTime: fi.ModTime().Format(time.RFC3339),
		})
	}

	return result, nil
}

// SessionMkdir creates directory for a session
func (s *FileService) SessionMkdir(ctx context.Context, sessionId, dir string) error {
	client, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return err
	}

	return client.MkdirAll(dir)
}

// SessionUpload uploads file for a session using streaming with progress tracking
func (s *FileService) SessionUpload(ctx context.Context, sessionId, targetPath string, file io.Reader, filename string, size int64) error {
	return s.SessionUploadWithID(ctx, "", sessionId, targetPath, file, filename, size)
}

// SessionUploadWithID uploads file for a session with custom transfer ID
func (s *FileService) SessionUploadWithID(ctx context.Context, transferID, sessionId, targetPath string, file io.Reader, filename string, size int64) error {
	client, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return err
	}

	var transfer *SessionFileTransfer
	transferManager := GetSessionTransferManager()

	if transferID != "" {
		// Use existing transfer or create new one
		transfer = transferManager.GetTransfer(transferID)
		if transfer == nil {
			transfer = transferManager.CreateTransferWithID(transferID, sessionId, filename, size, true)
			if transfer == nil {
				return fmt.Errorf("transfer ID already exists: %s", transferID)
			}
			// Set initial status
			transfer.UpdateProgress(0)
		}
	} else {
		// Create new transfer with auto-generated ID
		transfer = transferManager.CreateTransfer(sessionId, filename, size, true)
		transfer.UpdateProgress(0)
	}

	parentDir := filepath.Dir(targetPath)
	if parentDir != "" && parentDir != "." && parentDir != "/" {
		if _, err := client.Stat(parentDir); err != nil {
			if err := client.MkdirAll(parentDir); err != nil {
				transfer.SetError(fmt.Errorf("failed to create parent directory %s: %w", parentDir, err))
				return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
			}
		}
	}

	remoteFile, err := client.Create(targetPath)
	if err != nil {
		transfer.SetError(fmt.Errorf("failed to create remote file: %w", err))
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	progressWriter := NewProgressWriter(remoteFile, transfer)

	buffer := make([]byte, 32*1024)
	_, err = io.CopyBuffer(progressWriter, file, buffer)
	if err != nil {
		transfer.SetError(fmt.Errorf("failed to upload file: %w", err))
		return fmt.Errorf("failed to upload file: %w", err)
	}

	transfer.UpdateProgress(size)

	return nil
}

// SessionDownload downloads a single file for a session
func (s *FileService) SessionDownload(ctx context.Context, sessionId, filePath string) (io.ReadCloser, int64, error) {
	client, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return nil, 0, err
	}

	// Get file info for size
	fileInfo, err := client.Stat(filePath)
	if err != nil {
		return nil, 0, err
	}

	if fileInfo.IsDir() {
		return nil, 0, fmt.Errorf("cannot download directory as single file")
	}

	// Open file for reading
	reader, err := client.Open(filePath)
	if err != nil {
		return nil, 0, err
	}

	return reader, fileInfo.Size(), nil
}

// SessionDownloadMultiple downloads multiple files/directories as ZIP for a session
func (s *FileService) SessionDownloadMultiple(ctx context.Context, sessionId, dir string, filenames []string) (io.ReadCloser, string, int64, error) {
	client, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return nil, "", 0, err
	}

	// Validate and sanitize filenames
	var sanitizedFilenames []string
	for _, filename := range filenames {
		sanitized, err := sanitizeFilename(filename)
		if err != nil {
			return nil, "", 0, fmt.Errorf("invalid filename '%s': %v", filename, err)
		}
		sanitizedFilenames = append(sanitizedFilenames, sanitized)
	}

	// If only one file and it's not a directory, return directly
	if len(sanitizedFilenames) == 1 {
		fullPath := filepath.Join(dir, sanitizedFilenames[0])
		fileInfo, err := client.Stat(fullPath)
		if err != nil {
			return nil, "", 0, err
		}

		if !fileInfo.IsDir() {
			reader, err := client.Open(fullPath)
			if err != nil {
				return nil, "", 0, err
			}
			return reader, sanitizedFilenames[0], fileInfo.Size(), nil
		}
	}

	// Multiple files or contains directory, create ZIP
	return s.createZipArchive(client, dir, sanitizedFilenames)
}

// Session lifecycle management

// InitSessionFileClient initializes SFTP connection for a session
func (s *FileService) InitSessionFileClient(sessionId string, assetId, accountId int) error {
	return GetSessionFileManager().InitSessionSFTP(sessionId, assetId, accountId)
}

// CloseSessionFileClient closes SFTP connection for a session
func (s *FileService) CloseSessionFileClient(sessionId string) {
	GetSessionFileManager().CloseSessionSFTP(sessionId)
}

// IsSessionActive checks if session has active SFTP connection
func (s *FileService) IsSessionActive(sessionId string) bool {
	return GetSessionFileManager().IsSessionActive(sessionId)
}

func sanitizeFilename(filename string) (string, error) {
	// Remove any path traversal attempts
	if strings.Contains(filename, "..") ||
		strings.Contains(filename, "/") ||
		strings.Contains(filename, "\\") {
		return "", fmt.Errorf("invalid filename: path traversal detected")
	}

	// Remove null bytes and control characters
	if strings.ContainsAny(filename, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f") {
		return "", fmt.Errorf("invalid filename: control characters detected")
	}

	return filename, nil
}

// SessionFileTransfer represents a session-based file transfer
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

// SessionFileTransferProgress represents session transfer progress information
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

// SessionFileTransferManager manages session file transfers
type SessionFileTransferManager struct {
	transfers map[string]*SessionFileTransfer
	mutex     sync.RWMutex
}

var sessionTransferManager = &SessionFileTransferManager{
	transfers: make(map[string]*SessionFileTransfer),
}

// GetSessionTransferManager returns the global session transfer manager
func GetSessionTransferManager() *SessionFileTransferManager {
	return sessionTransferManager
}

// CreateTransfer creates a new session file transfer
func (m *SessionFileTransferManager) CreateTransfer(sessionID, filename string, size int64, isUpload bool) *SessionFileTransfer {
	return m.CreateTransferWithID("", sessionID, filename, size, isUpload)
}

// CreateTransferWithID creates a new session file transfer with custom ID
func (m *SessionFileTransferManager) CreateTransferWithID(transferID, sessionID, filename string, size int64, isUpload bool) *SessionFileTransfer {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var id string
	if transferID != "" {
		if _, exists := m.transfers[transferID]; exists {
			return nil
		}
		id = transferID
	} else {
		id = generateTransferID()
	}

	now := time.Now()

	transfer := &SessionFileTransfer{
		ID:        id,
		SessionID: sessionID,
		Filename:  filename,
		Size:      size,
		Status:    "pending",
		IsUpload:  isUpload,
		Created:   now,
		Updated:   now,
	}

	m.transfers[id] = transfer
	return transfer
}

// GetTransfer gets a transfer by ID
func (m *SessionFileTransferManager) GetTransfer(id string) *SessionFileTransfer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.transfers[id]
}

// GetTransfersBySession gets all transfers for a session
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

// GetSessionProgress gets progress information for all transfers in a session
func (m *SessionFileTransferManager) GetSessionProgress(sessionID string) []*SessionFileTransferProgress {
	transfers := m.GetTransfersBySession(sessionID)

	var progresses []*SessionFileTransferProgress
	for _, transfer := range transfers {
		progresses = append(progresses, transfer.GetProgress())
	}

	return progresses
}

// RemoveTransfer removes a transfer by ID
func (m *SessionFileTransferManager) RemoveTransfer(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.transfers, id)
}

// CleanupCompletedTransfers removes completed transfers older than specified duration
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

// UpdateProgress updates the transfer progress
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

// generateTransferID generates a unique transfer ID
func generateTransferID() string {
	return fmt.Sprintf("transfer_%d", time.Now().UnixNano())
}

// ProgressWriter wraps an io.Writer to track upload progress
type ProgressWriter struct {
	writer   io.Writer
	transfer *SessionFileTransfer
	written  int64
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(writer io.Writer, transfer *SessionFileTransfer) *ProgressWriter {
	return &ProgressWriter{
		writer:   writer,
		transfer: transfer,
	}
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		pw.transfer.SetError(err)
		return n, err
	}

	pw.written += int64(n)
	pw.transfer.UpdateProgress(pw.written)

	return n, nil
}

// =============================================================================
// Utility Methods (moved from controller)
// =============================================================================

// GetActionCode converts operation string to action code
func (s *FileService) GetActionCode(operation string) int {
	switch operation {
	case "upload":
		return model.FILE_ACTION_UPLOAD
	case "download":
		return model.FILE_ACTION_DOWNLOAD
	case "mkdir":
		return model.FILE_ACTION_MKDIR
	default:
		return model.FILE_ACTION_LS
	}
}

// ValidateAndNormalizePath validates and normalizes file paths to prevent directory traversal
func (s *FileService) ValidateAndNormalizePath(basePath, userPath string) (string, error) {
	// Clean the user-provided path
	cleanPath := filepath.Clean(userPath)

	// Ensure it doesn't contain directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path contains directory traversal: %s", userPath)
	}

	// Join with base path and clean again
	fullPath := filepath.Join(basePath, cleanPath)

	// Ensure the resulting path is still within the base directory
	relPath, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %s", userPath)
	}

	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path outside base directory: %s", userPath)
	}

	return fullPath, nil
}

// GetRDPDrivePath gets the RDP drive path with fallback options
func (s *FileService) GetRDPDrivePath(assetId int) (string, error) {
	var drivePath string

	// Priority 1: Get from environment variable (highest priority)
	drivePath = os.Getenv("ONETERM_RDP_DRIVE_PATH")

	// Priority 2: Use default path based on OS
	if drivePath == "" {
		if runtime.GOOS == "windows" {
			drivePath = filepath.Join("C:", "temp", "oneterm", "rdp")
		} else {
			drivePath = filepath.Join("/tmp", "oneterm", "rdp")
		}
	}

	// Create asset-specific subdirectory
	fullDrivePath := filepath.Join(drivePath, fmt.Sprintf("asset_%d", assetId))

	// Ensure directory exists
	if err := os.MkdirAll(fullDrivePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create RDP drive directory %s: %w", fullDrivePath, err)
	}

	return fullDrivePath, nil
}

// =============================================================================
// Transfer Progress Management (migrated from controller)
// =============================================================================

// FileTransferProgress represents unified progress tracking for SFTP and RDP transfers
type FileTransferProgress struct {
	TotalSize       int64
	TransferredSize int64
	Status          string // "transferring", "completed", "failed"
	Type            string // "sftp", "rdp"
}

// Progress tracking state
var (
	fileTransfers = make(map[string]*FileTransferProgress)
	transferMutex sync.RWMutex
)

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

// =============================================================================
// Permission Check Utilities
// =============================================================================

// RDP file upload with custom transfer ID
func UploadRDPFileStreamWithID(tunnel *guacd.Tunnel, transferID, sessionId, path string, reader io.Reader, totalSize int64) error {
	// Get session to extract asset ID
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	// Get drive path with proper fallback handling
	drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
	if err != nil {
		return fmt.Errorf("failed to get drive path: %w", err)
	}

	// Create transfer tracker
	var transfer *guacd.FileTransfer
	if transferID != "" {
		transfer, err = guacd.DefaultFileTransferManager.CreateUploadWithID(transferID, sessionId, filepath.Base(path), drivePath)
	} else {
		transfer, err = guacd.DefaultFileTransferManager.CreateUpload(sessionId, filepath.Base(path), drivePath)
	}

	if err != nil {
		return fmt.Errorf("failed to create transfer tracker: %w", err)
	}
	// Note: Don't remove transfer immediately - let it be cleaned up later so progress can be queried

	transfer.SetSize(totalSize)

	// Validate and construct full filesystem path
	fullPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	destDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer destFile.Close()

	progressWriter := NewRDPProgressWriter(destFile, transfer, transferID)

	buffer := make([]byte, 32*1024)
	written, err := io.CopyBuffer(progressWriter, reader, buffer)
	if err != nil {
		os.Remove(fullPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	if totalSize > 0 && written != totalSize {
		os.Remove(fullPath)
		// Mark as failed in unified tracking
		UpdateTransferProgress(transferID, 0, -1, "failed")
		return fmt.Errorf("file size mismatch: expected %d, wrote %d", totalSize, written)
	}

	// Mark as completed in unified tracking
	UpdateTransferProgress(transferID, 0, written, "completed")

	return nil
}

// Get RDP transfer progress by ID
func GetRDPTransferProgressById(transferId string) (interface{}, error) {
	progress, err := guacd.DefaultFileTransferManager.GetTransferProgress(transferId)
	if err != nil {
		return nil, err
	}
	return progress, nil
}

// RDP configuration check functions
func IsRDPDriveEnabled(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	driveEnabled := tunnel.Config.Parameters["enable-drive"] == "true"
	return driveEnabled
}

func IsRDPUploadAllowed(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	return tunnel.Config.Parameters["disable-upload"] != "true"
}

func IsRDPDownloadAllowed(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	return tunnel.Config.Parameters["disable-download"] != "true"
}

// RDP file listing functions
func RequestRDPFileList(tunnel *guacd.Tunnel, path string) ([]RDPFileInfo, error) {
	return RequestRDPFileListViaDirect(tunnel, path)
}

func RequestRDPFileListViaDirect(tunnel *guacd.Tunnel, path string) ([]RDPFileInfo, error) {
	// Get session to extract asset ID
	sessionId := tunnel.SessionId
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return nil, fmt.Errorf("session not found: %s", sessionId)
	}

	// Get drive path with proper fallback handling
	drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
	if err != nil {
		return nil, fmt.Errorf("failed to get drive path: %w", err)
	}

	// Validate and construct full filesystem path
	fullPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Read directory contents
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []RDPFileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries with errors
		}

		files = append(files, RDPFileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	return files, nil
}

// RDP file download functions
func DownloadRDPFile(tunnel *guacd.Tunnel, path string) (io.ReadCloser, int64, error) {
	// Get session to extract asset ID
	sessionId := tunnel.SessionId
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return nil, 0, fmt.Errorf("session not found: %s", sessionId)
	}

	// Get drive path with proper fallback handling
	drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get drive path: %w", err)
	}

	// Validate and construct full filesystem path
	fullPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, path)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists and is a file
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, 0, fmt.Errorf("file not found: %w", err)
	}

	if info.IsDir() {
		return nil, 0, fmt.Errorf("path is a directory, not a file")
	}

	// Open file for streaming (memory-efficient)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}

	return file, info.Size(), nil
}

func DownloadRDPMultiple(tunnel *guacd.Tunnel, dir string, filenames []string) (io.ReadCloser, string, int64, error) {
	var sanitizedFilenames []string
	for _, filename := range filenames {
		if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
			return nil, "", 0, fmt.Errorf("invalid filename: %s", filename)
		}
		sanitizedFilenames = append(sanitizedFilenames, filename)
	}

	if len(sanitizedFilenames) == 1 {
		fullPath := filepath.Join(dir, sanitizedFilenames[0])

		// Check if it's a directory or file
		sessionId := tunnel.SessionId
		onlineSession := gsession.GetOnlineSessionById(sessionId)
		if onlineSession == nil {
			return nil, "", 0, fmt.Errorf("session not found: %s", sessionId)
		}

		drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
		if err != nil {
			return nil, "", 0, fmt.Errorf("failed to get drive path: %w", err)
		}

		realPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, fullPath)
		if err != nil {
			return nil, "", 0, fmt.Errorf("invalid path: %w", err)
		}

		info, err := os.Stat(realPath)
		if err != nil {
			return nil, "", 0, fmt.Errorf("file not found: %w", err)
		}

		if info.IsDir() {
			// For directory, create a zip with directory contents
			return CreateRDPZip(tunnel, dir, sanitizedFilenames)
		} else {
			// For single file, download directly
			reader, fileSize, err := DownloadRDPFile(tunnel, fullPath)
			if err != nil {
				return nil, "", 0, err
			}
			return reader, sanitizedFilenames[0], fileSize, nil
		}
	}

	// Multiple files/directories - always create zip
	return CreateRDPZip(tunnel, dir, sanitizedFilenames)
}

func CreateRDPZip(tunnel *guacd.Tunnel, dir string, filenames []string) (io.ReadCloser, string, int64, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for _, filename := range filenames {
		fullPath := filepath.Join(dir, filename)
		err := AddToRDPZip(tunnel, zipWriter, fullPath, filename)
		if err != nil {
			zipWriter.Close()
			return nil, "", 0, fmt.Errorf("failed to add %s to zip: %w", filename, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, "", 0, fmt.Errorf("failed to close zip: %w", err)
	}

	downloadFilename := fmt.Sprintf("rdp_files_%s.zip", time.Now().Format("20060102_150405"))
	reader := io.NopCloser(bytes.NewReader(buf.Bytes()))
	return reader, downloadFilename, int64(buf.Len()), nil
}

func AddToRDPZip(tunnel *guacd.Tunnel, zipWriter *zip.Writer, fullPath, zipPath string) error {
	// Get session to extract asset ID
	sessionId := tunnel.SessionId
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	// Get drive path with proper fallback handling
	drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
	if err != nil {
		return fmt.Errorf("failed to get drive path: %w", err)
	}

	// Validate and construct full filesystem path
	realPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, fullPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	if info.IsDir() {
		// Add directory entries recursively
		entries, err := os.ReadDir(realPath)
		if err != nil {
			return fmt.Errorf("failed to read directory: %w", err)
		}

		// Create directory entry in zip if not empty
		if len(entries) == 0 {
			// Create empty directory entry
			dirHeader := &zip.FileHeader{
				Name:   zipPath + "/",
				Method: zip.Store,
			}
			_, err := zipWriter.CreateHeader(dirHeader)
			if err != nil {
				return fmt.Errorf("failed to create directory entry: %w", err)
			}
		} else {
			// Add all files in directory
			for _, entry := range entries {
				entryPath := filepath.Join(fullPath, entry.Name())
				entryZipPath := zipPath + "/" + entry.Name()
				err := AddToRDPZip(tunnel, zipWriter, entryPath, entryZipPath)
				if err != nil {
					return err
				}
			}
		}
	} else {
		// Add file to zip
		file, err := os.Open(realPath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		writer, err := zipWriter.Create(zipPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		// Stream file content to zip (memory-efficient)
		if _, err := io.Copy(writer, file); err != nil {
			return fmt.Errorf("failed to write file to zip: %w", err)
		}
	}

	return nil
}

func CreateRDPDirectory(tunnel *guacd.Tunnel, path string) error {
	// Get session to extract asset ID
	sessionId := tunnel.SessionId
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	// Get drive path with proper fallback handling
	drivePath, err := DefaultFileService.GetRDPDrivePath(onlineSession.AssetId)
	if err != nil {
		return fmt.Errorf("failed to get drive path: %w", err)
	}

	// Validate and construct full filesystem path
	fullPath, err := DefaultFileService.ValidateAndNormalizePath(drivePath, path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Create directory and any necessary parent directories
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
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

// =============================================================================
// SFTP Transfer Utilities (migrated from controller)
// =============================================================================

// AsyncTransferToTarget transfers file - unified function for both session and asset/account based transfers
func TransferToTarget(transferId, sessionIdOrCustom, tempFilePath, targetPath string, assetId, accountId int) error {
	// For session-based transfers, try to reuse existing SFTP connection first
	if assetId == 0 && accountId == 0 && sessionIdOrCustom != "" {
		return SessionBasedTransfer(transferId, sessionIdOrCustom, tempFilePath, targetPath)
	}

	// For asset/account-based transfers, fall back to creating new connection
	return AssetBasedTransfer(transferId, tempFilePath, targetPath, assetId, accountId)
}

// SessionBasedTransfer uses existing session SFTP connection for optimal performance
func SessionBasedTransfer(transferId, sessionId, tempFilePath, targetPath string) error {
	// Try to get existing SFTP client from session manager
	sftpClient, err := sessionFM.GetSessionSFTP(sessionId)
	if err != nil {
		// If no existing connection, create one
		onlineSession := gsession.GetOnlineSessionById(sessionId)
		if onlineSession == nil {
			return fmt.Errorf("session %s not found", sessionId)
		}

		// Initialize SFTP connection for this session
		if initErr := sessionFM.InitSessionSFTP(sessionId, onlineSession.AssetId, onlineSession.AccountId); initErr != nil {
			return fmt.Errorf("failed to initialize SFTP for session %s: %w", sessionId, initErr)
		}

		// Get the newly created SFTP client
		sftpClient, err = sessionFM.GetSessionSFTP(sessionId)
		if err != nil {
			return fmt.Errorf("failed to get SFTP client for session %s: %w", sessionId, err)
		}
	}

	// Use existing SFTP client for transfer (no need to close it as it's managed by SessionFileManager)
	return SftpUploadWithExistingClient(sftpClient, transferId, tempFilePath, targetPath)
}

// AssetBasedTransfer creates new connection for asset/account-based transfers (legacy)
func AssetBasedTransfer(transferId, tempFilePath, targetPath string, assetId, accountId int) error {
	var asset *model.Asset
	var account *model.Account
	var gateway *model.Gateway
	var err error

	// Asset/account based transfer
	asset, account, gateway, err = GetAAG(assetId, accountId)
	if err != nil {
		return fmt.Errorf("failed to get asset/account info: %w", err)
	}
	sessionId := fmt.Sprintf("upload_%d_%d_%d", assetId, accountId, time.Now().UnixNano())

	// Get SSH connection details
	ip, port, err := tunneling.Proxy(false, sessionId, "ssh", asset, gateway)
	if err != nil {
		return fmt.Errorf("failed to setup tunnel: %w", err)
	}

	auth, err := GetAuth(account)
	if err != nil {
		return fmt.Errorf("failed to get auth: %w", err)
	}

	// Create SSH client with maximum performance optimizations for SFTP
	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
		User:            account.Account,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
		// Ultra-high performance optimizations - fastest algorithms first
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-ctr",                    // Fastest for most CPUs with AES-NI
				"aes128-gcm@openssh.com",        // Hardware accelerated AEAD cipher
				"chacha20-poly1305@openssh.com", // Fast on ARM/systems without AES-NI
				"aes256-ctr",                    // Fallback high-performance option
			},
			MACs: []string{
				"hmac-sha2-256-etm@openssh.com", // Encrypt-then-MAC (fastest + most secure)
				"hmac-sha2-256",                 // Standard high-performance MAC
			},
			KeyExchanges: []string{
				"curve25519-sha256@libssh.org", // Modern elliptic curve (fastest)
				"curve25519-sha256",            // Equivalent modern KEX
				"ecdh-sha2-nistp256",           // Fast NIST curve fallback
			},
		},
		// Optimize connection algorithms for speed
		HostKeyAlgorithms: []string{
			"rsa-sha2-256", // Fast RSA with SHA-2
			"rsa-sha2-512", // Alternative fast RSA
			"ssh-ed25519",  // Modern EdDSA (very fast verification)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to connect SSH: %w", err)
	}
	defer sshClient.Close()

	// Use optimized SFTP to transfer file
	return SftpUploadWithProgress(sshClient, transferId, tempFilePath, targetPath)
}

// Progress writer for tracking transfer progress
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
		lastUpdate: time.Now(),
	}
}

func (pw *FileProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.written += int64(n)
	pw.updateBytes += int64(n)
	pw.updateTicker++

	// Update progress every 64KB bytes OR every 1000 write calls (reduces time.Now() overhead)
	if pw.updateBytes >= 65536 || pw.updateTicker >= 1000 {
		now := time.Now()
		// Only update if enough time has passed (reduce lock contention)
		if pw.updateBytes >= 65536 || now.Sub(pw.lastUpdate) >= 50*time.Millisecond {
			UpdateTransferProgress(pw.transferId, 0, pw.written, "")
			pw.lastUpdate = now
			pw.updateBytes = 0
			pw.updateTicker = 0
		}
	}

	return n, nil
}

// SftpUploadWithProgress uploads file using optimized SFTP protocol with accurate progress tracking
func SftpUploadWithProgress(client *ssh.Client, transferId, localPath, remotePath string) error {
	// Create SFTP client with maximum performance settings
	sftpClient, err := sftp.NewClient(client,
		sftp.MaxPacket(1024*32),               // 32KB packets - maximum safe size for most servers
		sftp.MaxConcurrentRequestsPerFile(64), // High concurrency for maximum throughput
		sftp.UseConcurrentReads(true),         // Enable concurrent reads for better performance
		sftp.UseConcurrentWrites(true),        // Enable concurrent writes for better performance
	)
	if err != nil {
		logger.L().Error("Failed to create SFTP client",
			zap.String("transferId", transferId),
			zap.Error(err))
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create parent directory on remote if needed
	parentDir := filepath.Dir(remotePath)
	if parentDir != "" && parentDir != "." && parentDir != "/" {
		if err := sftpClient.MkdirAll(parentDir); err != nil {
			logger.L().Warn("Failed to create parent directory via SFTP", zap.Error(err))
		}
	}

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create progress tracking writer with SFTP-specific optimizations
	progressWriter := NewFileProgressWriter(remoteFile, transferId)

	// Transfer file content with ultra-high performance buffer for SFTP
	// Use 2MB buffer to minimize round trips and maximize throughput
	buffer := make([]byte, 2*1024*1024) // 2MB buffer for ultra-high SFTP performance

	// Manual optimized copy loop to avoid io.CopyBuffer overhead
	var transferred int64
	for {
		n, readErr := localFile.Read(buffer)
		if n > 0 {
			written, writeErr := progressWriter.Write(buffer[:n])
			transferred += int64(written)
			if writeErr != nil {
				err = writeErr
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break // Normal end of file
			}
			err = readErr
			break
		}
	}
	if err != nil {
		logger.L().Error("SFTP file transfer failed during copy",
			zap.String("transferId", transferId),
			zap.Int64("transferred", transferred),
			zap.Int64("fileSize", fileInfo.Size()),
			zap.Error(err))
		return fmt.Errorf("failed to transfer file content via SFTP: %w", err)
	}

	// Force final progress update
	UpdateTransferProgress(transferId, 0, transferred, "")
	return nil
}

// SftpUploadWithExistingClient uploads file using existing SFTP client with accurate progress tracking
func SftpUploadWithExistingClient(client *sftp.Client, transferId, localPath, remotePath string) error {
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create parent directory on remote if needed
	parentDir := filepath.Dir(remotePath)
	if parentDir != "" && parentDir != "." && parentDir != "/" {
		if err := client.MkdirAll(parentDir); err != nil {
			logger.L().Warn("Failed to create parent directory via SFTP", zap.Error(err))
		}
	}

	// Create remote file
	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create progress tracking writer with SFTP-specific optimizations
	progressWriter := NewFileProgressWriter(remoteFile, transferId)

	// Transfer file content with ultra-high performance buffer for SFTP
	// Use 2MB buffer to minimize round trips and maximize throughput
	buffer := make([]byte, 2*1024*1024) // 2MB buffer for ultra-high SFTP performance

	// Manual optimized copy loop to avoid io.CopyBuffer overhead
	var transferred int64
	for {
		n, readErr := localFile.Read(buffer)
		if n > 0 {
			written, writeErr := progressWriter.Write(buffer[:n])
			transferred += int64(written)
			if writeErr != nil {
				err = writeErr
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break // Normal end of file
			}
			err = readErr
			break
		}
	}
	if err != nil {
		logger.L().Error("SFTP file transfer failed during copy",
			zap.String("transferId", transferId),
			zap.Int64("transferred", transferred),
			zap.Int64("fileSize", fileInfo.Size()),
			zap.Error(err))
		return fmt.Errorf("failed to transfer file content via SFTP: %w", err)
	}

	// Force final progress update
	UpdateTransferProgress(transferId, 0, transferred, "")
	return nil
}

// RecordFileHistory records file operation history with unified interface
// This method abstracts away the differences between SFTP, RDP and other file operations
func (s *FileService) RecordFileHistory(ctx context.Context, operation, dir, filename string, assetId, accountId int, sessionId ...string) error {
	// Extract user information from context
	currentUser, err := acl.GetSessionFromCtx(ctx)
	if err != nil || currentUser == nil {
		// If no user context, still record the operation but with empty user info
		logger.L().Warn("No user context found when recording file history", zap.String("operation", operation))
	}

	var uid int
	var userName string
	var clientIP string

	if currentUser != nil {
		uid = currentUser.GetUid()
		userName = currentUser.GetUserName()
	}

	// Get client IP from gin context
	if ginCtx, ok := ctx.(*gin.Context); ok {
		clientIP = ginCtx.ClientIP()
	}

	history := &model.FileHistory{
		Uid:       uid,
		UserName:  userName,
		AssetId:   assetId,
		AccountId: accountId,
		ClientIp:  clientIP,
		Action:    s.GetActionCode(operation),
		Dir:       dir,
		Filename:  filename,
	}

	if err := s.AddFileHistory(ctx, history); err != nil {
		// Log error details including sessionId if provided for debugging
		sessionIdStr := ""
		if len(sessionId) > 0 {
			sessionIdStr = sessionId[0]
		}
		logger.L().Error("Failed to record file history",
			zap.Error(err),
			zap.String("operation", operation),
			zap.String("sessionId", sessionIdStr),
			zap.Any("history", history))
		return err
	}

	return nil
}

// RecordFileHistoryBySession records file operation history using sessionId to get asset/account info
// This is a convenience method for session-based operations where we only have sessionId
func (s *FileService) RecordFileHistoryBySession(ctx context.Context, sessionId, operation, path string) error {
	// Get session info to extract asset and account information
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		logger.L().Warn("Cannot record file history: session not found", zap.String("sessionId", sessionId))
		return fmt.Errorf("session not found: %s", sessionId)
	}

	// Extract directory and filename from path
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	return s.RecordFileHistory(ctx, operation, dir, filename, onlineSession.AssetId, onlineSession.AccountId, sessionId)
}

// SetError sets an error for the transfer
func (t *SessionFileTransfer) SetError(err error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.Status = "failed"
	t.Error = err.Error()
	t.Updated = time.Now()
}
