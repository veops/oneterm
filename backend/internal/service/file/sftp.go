package file

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

// =============================================================================
// SFTP Operations - Managers defined in parent file service
// =============================================================================

// =============================================================================
// SFTP Upload/Download Operations with Progress Tracking
// =============================================================================

// TransferToTarget handles transfer routing (session-based or asset-based)
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
	sessionFM := GetSessionFileManager()
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
	asset, account, gateway, err := repository.GetAAG(assetId, accountId)
	if err != nil {
		return fmt.Errorf("failed to get asset/account info: %w", err)
	}
	sessionId := fmt.Sprintf("upload_%d_%d_%d", assetId, accountId, time.Now().UnixNano())

	// Get SSH connection details
	ip, port, err := tunneling.Proxy(false, sessionId, "ssh", asset, gateway)
	if err != nil {
		return fmt.Errorf("failed to setup tunnel: %w", err)
	}

	auth, err := repository.GetAuth(account)
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

	// Create progress tracking writer
	progressWriter := NewFileProgressWriter(remoteFile, transferId)

	// Transfer file content with ultra-high performance buffer for SFTP
	// Use 2MB buffer to minimize round trips and maximize throughput
	buffer := make([]byte, 2*1024*1024) // 2MB buffer for ultra-high SFTP performance
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
		logger.L().Error("SFTP file transfer failed",
			zap.String("transferId", transferId),
			zap.Int64("transferred", transferred),
			zap.Int64("fileSize", fileInfo.Size()),
			zap.Error(err))
		return fmt.Errorf("failed to transfer file: %w", err)
	}

	// Force final progress update
	UpdateTransferProgress(transferId, 0, transferred, "")
	logger.L().Info("SFTP file transfer completed",
		zap.String("transferId", transferId),
		zap.String("remotePath", remotePath),
		zap.Int64("size", transferred))

	return nil
}

// =============================================================================
// SFTP Download Operations with ZIP Support
// =============================================================================

// SftpDownloadMultiple downloads multiple files as ZIP or single file
func SftpDownloadMultiple(ctx context.Context, assetId, accountId int, dir string, filenames []string) (io.ReadCloser, string, int64, error) {
	cli, err := GetFileManager().GetFileClient(assetId, accountId)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to get SFTP client: %w", err)
	}

	if len(filenames) == 1 {
		// Single file download
		fullPath := filepath.Join(dir, filenames[0])
		file, err := cli.Open(fullPath)
		if err != nil {
			return nil, "", 0, fmt.Errorf("failed to open file %s: %w", fullPath, err)
		}

		// Get file size
		info, err := cli.Stat(fullPath)
		if err != nil {
			file.Close()
			return nil, "", 0, fmt.Errorf("failed to get file info: %w", err)
		}

		return file, filenames[0], info.Size(), nil
	}

	// Multiple files - use the common ZIP creation logic from FileService
	fileService := &FileService{}
	return fileService.createZipArchive(cli, dir, filenames)
}

// =============================================================================
// SFTP Progress Writers
// =============================================================================

// SftpProgressWriter tracks SFTP transfer progress
type SftpProgressWriter struct {
	writer       io.Writer
	transferId   string
	written      int64
	lastUpdate   time.Time
	updateBytes  int64 // Bytes written since last progress update
	updateTicker int64 // Simple counter to reduce time.Now() calls
}

// NewSftpProgressWriter creates a new SFTP progress writer
func NewSftpProgressWriter(writer io.Writer, transferId string) *SftpProgressWriter {
	return &SftpProgressWriter{
		writer:     writer,
		transferId: transferId,
		lastUpdate: time.Now(),
	}
}

func (pw *SftpProgressWriter) Write(p []byte) (int, error) {
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
