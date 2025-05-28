package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/tunneling"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

var (
	fm = &FileManager{
		sftps:    map[string]*sftp.Client{},
		lastTime: map[string]time.Time{},
		mtx:      sync.Mutex{},
	}

	// Global file service instance
	DefaultFileService IFileService
)

// InitFileService initializes the global file service
func InitFileService() {
	repo := repository.NewFileRepository(dbpkg.DB)
	DefaultFileService = NewFileService(repo)
}

func init() {
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

	cli, err = sftp.NewClient(sshCli)
	fm.sftps[key] = cli

	return
}

// File service interface
type IFileService interface {
	ReadDir(ctx context.Context, assetId, accountId int, dir string) ([]fs.FileInfo, error)
	MkdirAll(ctx context.Context, assetId, accountId int, dir string) error
	Create(ctx context.Context, assetId, accountId int, path string) (io.WriteCloser, error)
	Open(ctx context.Context, assetId, accountId int, path string) (io.ReadCloser, error)
	Stat(ctx context.Context, assetId, accountId int, path string) (fs.FileInfo, error)
	DownloadMultiple(ctx context.Context, assetId, accountId int, dir string, filenames []string) (io.ReadCloser, string, int64, error)
	AddFileHistory(ctx context.Context, history *model.FileHistory) error
	GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error)

	// RDP file transfer methods
	RDPReadDir(ctx context.Context, sessionId, dir string) ([]fs.FileInfo, error)
	RDPMkdirAll(ctx context.Context, sessionId, dir string) error
	RDPUploadFile(ctx context.Context, sessionId, filename string, content []byte) error
	RDPDownloadFile(ctx context.Context, sessionId, filename string) ([]byte, error)
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
	// Open remote file
	file, err := cli.Open(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create file in ZIP
	zipFile, err := zipWriter.Create(relativePath)
	if err != nil {
		return err
	}

	// Copy file content
	_, err = io.Copy(zipFile, file)
	return err
}

// addDirToZip recursively adds a directory to ZIP archive
func (s *FileService) addDirToZip(cli *sftp.Client, zipWriter *zip.Writer, fullPath, relativePath string) error {
	// Read directory contents
	entries, err := cli.ReadDir(fullPath)
	if err != nil {
		return err
	}

	// If directory is empty, create directory entry
	if len(entries) == 0 {
		_, err := zipWriter.Create(relativePath + "/")
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

// GetFileHistory gets file history records
func (s *FileService) GetFileHistory(ctx context.Context, filters map[string]interface{}) ([]*model.FileHistory, int64, error) {
	return s.repo.GetFileHistory(ctx, filters)
}

// RDP file transfer methods implementation

// RDPReadDir reads directory contents for RDP session
func (s *FileService) RDPReadDir(ctx context.Context, sessionId, dir string) ([]fs.FileInfo, error) {
	// Get session tunnel to access file transfer manager
	tunnel := tunneling.GetTunnelBySessionId(sessionId)
	if tunnel == nil {
		return nil, fmt.Errorf("session not found: %s", sessionId)
	}

	// For RDP sessions, we need to check if drive is enabled
	// This would need to be implemented based on the actual session configuration
	// For now, return an error indicating RDP file operations need to be handled differently
	return nil, fmt.Errorf("RDP file operations should be handled through Guacamole protocol")
}

// RDPMkdirAll creates directory for RDP session
func (s *FileService) RDPMkdirAll(ctx context.Context, sessionId, dir string) error {
	tunnel := tunneling.GetTunnelBySessionId(sessionId)
	if tunnel == nil {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	return fmt.Errorf("RDP file operations should be handled through Guacamole protocol")
}

// RDPUploadFile uploads file for RDP session
func (s *FileService) RDPUploadFile(ctx context.Context, sessionId, filename string, content []byte) error {
	tunnel := tunneling.GetTunnelBySessionId(sessionId)
	if tunnel == nil {
		return fmt.Errorf("session not found: %s", sessionId)
	}

	return fmt.Errorf("RDP file operations should be handled through Guacamole protocol")
}

// RDPDownloadFile downloads file for RDP session
func (s *FileService) RDPDownloadFile(ctx context.Context, sessionId, filename string) ([]byte, error) {
	tunnel := tunneling.GetTunnelBySessionId(sessionId)
	if tunnel == nil {
		return nil, fmt.Errorf("session not found: %s", sessionId)
	}

	return nil, fmt.Errorf("RDP file operations should be handled through Guacamole protocol")
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
