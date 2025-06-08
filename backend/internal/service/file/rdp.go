package file

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/veops/oneterm/internal/guacd"
	gsession "github.com/veops/oneterm/internal/session"
)

// RDP file operation functions

// NewRDPProgressWriter creates a new RDP progress writer
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
	UpdateTransferProgress(pw.transferId, 0, pw.written, "transferring")

	return n, nil
}

// IsRDPDriveEnabled checks if RDP drive is enabled
func IsRDPDriveEnabled(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	driveEnabled := tunnel.Config.Parameters["enable-drive"] == "true"
	return driveEnabled
}

// IsRDPUploadAllowed checks if RDP upload is allowed
func IsRDPUploadAllowed(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	return tunnel.Config.Parameters["disable-upload"] != "true"
}

// IsRDPDownloadAllowed checks if RDP download is allowed
func IsRDPDownloadAllowed(tunnel *guacd.Tunnel) bool {
	if tunnel == nil || tunnel.Config == nil {
		return false
	}
	return tunnel.Config.Parameters["disable-download"] != "true"
}

// RequestRDPFileList gets file list for RDP session
func RequestRDPFileList(tunnel *guacd.Tunnel, path string) ([]RDPFileInfo, error) {
	// Implementation placeholder - this would need to be implemented based on Guacamole protocol
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

// DownloadRDPFile downloads a single file from RDP session
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

// DownloadRDPMultiple downloads multiple files from RDP session as ZIP
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

// CreateRDPZip creates a ZIP archive of multiple RDP files
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

// AddToRDPZip adds a file or directory to the ZIP archive
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

// CreateRDPDirectory creates a directory in RDP session
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

	// Create directory with proper permissions
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Send refresh notification to RDP session
	NotifyRDPDirectoryRefresh(sessionId)

	return nil
}

// UploadRDPFileStreamWithID uploads file to RDP session with progress tracking
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

	// CRITICAL: Explicitly close and sync file before marking as completed
	// This ensures the file is fully written to disk and visible to mounted containers
	if err := destFile.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	// Clear macOS extended attributes that might interfere with Docker volume mounting
	if runtime.GOOS == "darwin" {
		// Clear attributes for the file and parent directory
		exec.Command("xattr", "-c", fullPath).Run()
		exec.Command("xattr", "-c", filepath.Dir(fullPath)).Run()
	}

	// Mark as completed in unified tracking
	UpdateTransferProgress(transferID, 0, written, "completed")

	// Send refresh notification to frontend with a slight delay
	// This ensures the file system operations are fully completed
	go func() {
		time.Sleep(500 * time.Millisecond)
		NotifyRDPDirectoryRefresh(sessionId)
	}()

	return nil
}

// GetRDPTransferProgressById gets RDP transfer progress by ID
func GetRDPTransferProgressById(transferId string) (interface{}, error) {
	progress, err := guacd.DefaultFileTransferManager.GetTransferProgress(transferId)
	if err != nil {
		return nil, err
	}
	return progress, nil
}

// NotifyRDPDirectoryRefresh sends F5 key to refresh Windows Explorer
func NotifyRDPDirectoryRefresh(sessionId string) {
	// Get the active session and tunnel
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return
	}

	tunnel := onlineSession.GuacdTunnel
	if tunnel == nil {
		return
	}

	// Send F5 key to refresh Windows Explorer
	// F5 key code: 65474
	f5DownInstruction := guacd.NewInstruction("key", "65474", "1")
	if _, err := tunnel.WriteInstruction(f5DownInstruction); err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond)
	f5UpInstruction := guacd.NewInstruction("key", "65474", "0")
	tunnel.WriteInstruction(f5UpInstruction)
}
