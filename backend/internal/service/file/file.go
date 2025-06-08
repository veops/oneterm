package file

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
)

// Global file service instance
var DefaultFileService IFileService

// InitFileService initializes the global file service
func InitFileService() {
	DefaultFileService = NewFileService(&FileRepository{
		db: dbpkg.DB,
	})
}

func init() {
	// Legacy file manager cleanup
	go func() {
		tk := time.NewTicker(time.Minute)
		for {
			<-tk.C
			func() {
				GetFileManager().mtx.Lock()
				defer GetFileManager().mtx.Unlock()
				for k, v := range GetFileManager().lastTime {
					if v.Before(time.Now().Add(time.Minute * 10)) {
						delete(GetFileManager().sftps, k)
						delete(GetFileManager().lastTime, k)
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

// NewFileService creates a new file service instance
func NewFileService(repo IFileRepository) IFileService {
	return &FileService{
		repo: repo,
	}
}

// Legacy asset-based operations
func (s *FileService) ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.ReadDir(dir)
}

func (s *FileService) MkdirAll(ctx context.Context, assetId, accountId int, dir string) error {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return err
	}
	return cli.MkdirAll(dir)
}

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

func (s *FileService) Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, err
	}
	return cli.Open(path)
}

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

// Session-based operations
func (s *FileService) SessionLS(ctx context.Context, sessionId, dir string) ([]FileInfo, error) {
	cli, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return nil, err
	}

	entries, err := cli.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var fileInfos []FileInfo
	for _, entry := range entries {
		var target string
		if entry.Mode()&fs.ModeSymlink != 0 {
			linkPath := filepath.Join(dir, entry.Name())
			if linkTarget, err := cli.ReadLink(linkPath); err == nil {
				target = linkTarget
			}
		}

		fileInfos = append(fileInfos, FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    entry.Size(),
			Mode:    entry.Mode().String(),
			IsLink:  entry.Mode()&fs.ModeSymlink != 0,
			Target:  target,
			ModTime: entry.ModTime().Format(time.RFC3339),
		})
	}

	return fileInfos, nil
}

func (s *FileService) SessionMkdir(ctx context.Context, sessionId, dir string) error {
	cli, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return err
	}
	return cli.MkdirAll(dir)
}

func (s *FileService) SessionUpload(ctx context.Context, sessionId, targetPath string, file io.Reader, filename string, size int64) error {
	return s.SessionUploadWithID(ctx, "", sessionId, targetPath, file, filename, size)
}

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

func (s *FileService) SessionDownload(ctx context.Context, sessionId, filePath string) (io.ReadCloser, int64, error) {
	cli, err := GetSessionFileManager().GetSessionSFTP(sessionId)
	if err != nil {
		return nil, 0, err
	}

	// Get file info
	info, err := cli.Stat(filePath)
	if err != nil {
		return nil, 0, err
	}

	// Open file
	file, err := cli.Open(filePath)
	if err != nil {
		return nil, 0, err
	}

	return file, info.Size(), nil
}

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
func (s *FileService) InitSessionFileClient(sessionId string, assetId, accountId int) error {
	return GetSessionFileManager().InitSessionSFTP(sessionId, assetId, accountId)
}

func (s *FileService) CloseSessionFileClient(sessionId string) {
	GetSessionFileManager().CloseSessionSFTP(sessionId)
}

func (s *FileService) IsSessionActive(sessionId string) bool {
	return GetSessionFileManager().IsSessionActive(sessionId)
}

// Progress tracking
func (s *FileService) GetSessionTransferProgress(ctx context.Context, sessionId string) ([]*SessionFileTransferProgress, error) {
	return GetSessionTransferManager().GetSessionProgress(sessionId), nil
}

func (s *FileService) GetTransferProgress(ctx context.Context, transferId string) (*SessionFileTransferProgress, error) {
	transfer := GetSessionTransferManager().GetTransfer(transferId)
	if transfer == nil {
		return nil, fmt.Errorf("transfer not found")
	}
	return transfer.GetProgress(), nil
}

// File history operations
func (s *FileService) AddFileHistory(ctx context.Context, history *model.FileHistory) error {
	if s.repo == nil {
		return fmt.Errorf("repository not initialized")
	}
	return s.repo.AddFileHistory(ctx, history)
}

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

// Utility methods
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

	// Clear macOS extended attributes that might interfere with Docker volume mounting
	if runtime.GOOS == "darwin" {
		// Clear attributes for the directory and all its contents
		exec.Command("find", fullDrivePath, "-exec", "xattr", "-c", "{}", ";").Run()
	}

	return fullDrivePath, nil
}

// Simple FileRepository implementation
type FileRepository struct {
	db *gorm.DB
}

func (r *FileRepository) AddFileHistory(ctx context.Context, history *model.FileHistory) error {
	return r.db.Create(history).Error
}

func (r *FileRepository) BuildFileHistoryQuery(ctx *gin.Context) *gorm.DB {
	return r.db.Model(&model.FileHistory{})
}
