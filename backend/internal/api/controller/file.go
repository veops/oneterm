package controller

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	MaxMemoryForParsing              = 32 << 20    // 32MB for multipart parsing
	MaxFileSize                      = 10240 << 20 // 10GB max file size
	MaxFileSizeForInMemoryProcessing = 64 << 20    // 64MB for in-memory processing
)

// GetFileHistory godoc
//
//	@Tags		file
//	@Param		page_index	query		int		true	"page_index"
//	@Param		page_size	query		int		true	"page_size"
//	@Param		search		query		string	false	"search"
//	@Param		action		query		int		false	"saction"
//	@Param		start		query		string	false	"start, RFC3339"
//	@Param		end			query		string	false	"end, RFC3339"
//	@Param		uid			query		int		false	"uid"
//	@Param		asset_id	query		int		false	"asset id"
//	@Param		account_id	query		int		false	"account id"
//	@Param		client_ip	query		string	false	"client_ip"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Session}}
//	@Router		/file/history [get]
func (c *Controller) GetFileHistory(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	db := service.DefaultFileService.BuildFileHistoryQuery(ctx)

	// Apply user permissions - non-admin users can only see their own history
	if !acl.IsAdmin(currentUser) {
		db = db.Where("uid = ?", currentUser.Uid)
	}

	doGet[*model.FileHistory](ctx, false, db, "")
}

// FileLS godoc
//
//	@Tags		file
//	@Param		asset_id	path		int		true	"asset_id"
//	@Param		account_id	path		int		true	"account_id"
//	@Param		dir			query		string	true	"dir"
//	@Param		show_hidden	query		bool	false	"show hidden files (default: false)"
//	@Success	200			{object}	HttpResponse
//	@Router		/file/ls/:asset_id/:account_id [GET]
func (c *Controller) FileLS(ctx *gin.Context) {
	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	info, err := service.DefaultFileService.ReadDir(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir"))
	if err != nil {
		if service.IsPermissionError(err) {
			ctx.AbortWithError(http.StatusForbidden, fmt.Errorf("permission denied"))
		} else {
			ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		}
		return
	}

	// Filter hidden files unless show_hidden is true
	showHidden := cast.ToBool(ctx.Query("show_hidden"))
	if !showHidden {
		info = lo.Filter(info, func(f fs.FileInfo, _ int) bool {
			return !strings.HasPrefix(f.Name(), ".")
		})
	}

	res := &ListData{
		Count: int64(len(info)),
		List: lo.Map(info, func(f fs.FileInfo, _ int) any {
			var target string
			if f.Mode()&os.ModeSymlink != 0 {
				cli, err := service.GetFileManager().GetFileClient(sess.Session.AssetId, sess.Session.AccountId)
				if err == nil {
					linkPath := filepath.Join(ctx.Query("dir"), f.Name())
					if linkTarget, err := cli.ReadLink(linkPath); err == nil {
						target = linkTarget
					}
				}
			}
			return &service.FileInfo{
				Name:    f.Name(),
				IsDir:   f.IsDir(),
				Size:    f.Size(),
				Mode:    f.Mode().String(),
				IsLink:  f.Mode()&os.ModeSymlink != 0,
				Target:  target,
				ModTime: f.ModTime().Format(time.RFC3339),
			}
		}),
	}
	ctx.JSON(http.StatusOK, NewHttpResponseWithData(res))
}

// FileMkdir godoc
//
//	@Tags		file
//	@Param		asset_id	path		int		true	"asset_id"
//	@Param		account_id	path		int		true	"account_id"
//	@Param		dir			query		string	true	"dir "
//	@Success	200			{object}	HttpResponse
//	@Router		/file/mkdir/:asset_id/:account_id [post]
func (c *Controller) FileMkdir(ctx *gin.Context) {
	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	if err := service.DefaultFileService.MkdirAll(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir")); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Record file history using unified method
	if err := service.DefaultFileService.RecordFileHistory(ctx, "mkdir", ctx.Query("dir"), "", sess.Session.AssetId, sess.Session.AccountId); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// FileUpload godoc
//
//	@Tags		file
//	@Param		asset_id		path		int		true	"asset_id"
//	@Param		account_id		path		int		true	"account_id"
//	@Param		dir				query		string	false	"target directory path (default: /tmp)"
//	@Param		transfer_id		query		string	false	"Custom transfer ID for progress tracking (frontend generated)"
//	@Accept		multipart/form-data
//	@Param		file			formData	file	true	"file to upload"
//	@Success	200				{object}	HttpResponse
//	@Router		/file/upload/:asset_id/:account_id [post]
func (c *Controller) FileUpload(ctx *gin.Context) {
	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Get transfer_id from URL query parameters (non-blocking)
	frontendTransferId := ctx.Query("transfer_id")
	targetDir := ctx.DefaultQuery("dir", "/tmp")

	// Use frontend provided transfer_id or generate new one
	var transferId string
	if frontendTransferId != "" {
		transferId = frontendTransferId
	} else {
		transferId = fmt.Sprintf("%d-%d-%d", sess.Session.AssetId, sess.Session.AccountId, time.Now().UnixNano())
	}

	service.CreateTransferProgress(transferId, "sftp")

	// Parse multipart form
	if err := ctx.Request.ParseMultipartForm(MaxMemoryForParsing); err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Failed to parse multipart form: %v", err),
		})
		return
	}

	// Get uploaded file
	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Failed to get uploaded file: %v", err),
		})
		return
	}
	defer file.Close()

	filename := fileHeader.Filename
	fileSize := fileHeader.Size

	if fileSize > MaxFileSize {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("File size %d bytes exceeds limit of %d bytes", fileSize, MaxFileSize),
		})
		return
	}

	// Update transfer progress with file size
	service.UpdateTransferProgress(transferId, fileSize, 0, "")

	targetPath := filepath.Join(targetDir, filename)

	// Phase 1: Save file to server temp directory first
	tempDir := filepath.Join(os.TempDir(), "oneterm-uploads", fmt.Sprintf("%d-%d", sess.Session.AssetId, sess.Session.AccountId))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		})
		return
	}

	tempFilePath := filepath.Join(tempDir, filename)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to create temp file: %v", err),
		})
		return
	}

	// Copy uploaded file to temp location
	written, err := io.Copy(tempFile, file)
	tempFile.Close()

	if err != nil {
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to save file: %v", err),
		})
		return
	}

	if written != fileSize {
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("File size mismatch: expected %d, got %d", fileSize, written),
		})
		return
	}

	// Phase 2: Transfer to target machine using SFTP (synchronous)
	service.UpdateTransferProgress(transferId, fileSize, 0, "transferring")

	if err := service.TransferToTarget(transferId, "", tempFilePath, targetPath, sess.Session.AssetId, sess.Session.AccountId); err != nil {
		service.UpdateTransferProgress(transferId, 0, -1, "failed")
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("File transfer failed: %v", err),
		})
		return
	}

	// Mark transfer as completed
	service.UpdateTransferProgress(transferId, 0, -1, "completed")

	// Clean up temp file after successful transfer
	os.Remove(tempFilePath)

	// Record file history using unified method
	if err := service.DefaultFileService.RecordFileHistory(ctx, "upload", targetDir, filename, sess.Session.AssetId, sess.Session.AccountId); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Return success response after transfer completion
	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "File uploaded successfully",
		Data: gin.H{
			"filename":    filename,
			"path":        targetPath,
			"size":        fileSize,
			"transfer_id": transferId,
			"status":      "completed",
		},
	})

	// Clean up progress record after a short delay
	go func() {
		time.Sleep(30 * time.Second) // Keep for 30 seconds for any delayed queries
		service.CleanupTransferProgress(transferId, 0)
	}()
}

// FileDownload godoc
//
//	@Tags		file
//	@Param		asset_id	path		int		true	"asset_id"
//	@Param		account_id	path		int		true	"account_id"
//	@Param		dir			query		string	true	"dir"
//	@Param		names	query		string	true	"names (comma-separated for multiple files)"
//	@Success	200			{object}	HttpResponse
//	@Router		/file/download/:asset_id/:account_id [get]
func (c *Controller) FileDownload(ctx *gin.Context) {
	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	filenameParam := ctx.Query("names")
	if filenameParam == "" {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "names parameter is required"}})
		return
	}

	filenames := lo.Filter(
		lo.Map(strings.Split(filenameParam, ","), func(name string, _ int) string {
			return strings.TrimSpace(name)
		}),
		func(name string, _ int) bool {
			return name != ""
		},
	)

	if len(filenames) == 0 {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": "no valid filenames provided"}})
		return
	}

	reader, downloadFilename, fileSize, err := service.DefaultFileService.DownloadMultiple(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir"), filenames)
	if err != nil {
		if service.IsPermissionError(err) {
			ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{"err": err}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		}
		return
	}
	defer reader.Close()

	// Record file operation history using unified method
	if err := service.DefaultFileService.RecordFileHistory(ctx, "download", ctx.Query("dir"), strings.Join(filenames, ","), sess.Session.AssetId, sess.Session.AccountId); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Set response headers for file download
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", downloadFilename))
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")

	// Set content length if known
	if fileSize > 0 {
		ctx.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	}

	// Stream file content directly to response
	ctx.Status(http.StatusOK)

	// Use streaming copy with buffer to handle large files efficiently
	buffer := make([]byte, 32*1024) // 32KB buffer for optimal performance
	_, err = io.CopyBuffer(ctx.Writer, reader, buffer)
	if err != nil {
		logger.L().Error("File transfer failed", zap.Error(err))
	}
}

// RDP File Transfer Methods

// RDPFileList lists files in RDP session drive
// @Summary List RDP session files
// @Description Get file list for RDP session drive
// @Tags RDP File
// @Param session_id path string true "Session ID"
// @Param path query string false "Directory path"
// @Success	200			{object}	HttpResponse
// @Router	/rdp/sessions/{session_id}/files [get]
func (c *Controller) RDPFileList(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	path := ctx.DefaultQuery("path", "/")

	tunnel, err := c.validateRDPAccess(ctx, sessionId)
	if err != nil {
		if strings.Contains(err.Error(), "permission") {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: err.Error(),
			})
		} else {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
		}
		return
	}

	// Check if RDP drive is enabled
	if !service.IsRDPDriveEnabled(tunnel) {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "RDP drive is not enabled for this session",
		})
		return
	}

	// Send file list request through Guacamole protocol
	files, err := service.RequestRDPFileList(tunnel, path)
	if err != nil {
		logger.L().Error("Failed to get RDP file list", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to get file list",
		})
		return
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "ok",
		Data:    files,
	})
}

// RDPFileUpload uploads file to RDP session drive
// @Summary Upload file to RDP session
// @Description Upload file to RDP session drive
// @Tags RDP File
// @Accept multipart/form-data
// @Param session_id path string true "Session ID"
// @Param transfer_id query string false "Custom transfer ID for progress tracking (frontend generated)"
// @Param path query string false "Target directory path"
// @Param file formData file true "File to upload"
// @Success 200 {object} HttpResponse
// @Router /rdp/sessions/{session_id}/files/upload [post]
func (c *Controller) RDPFileUpload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	// Get transfer_id from URL query parameters IMMEDIATELY (non-blocking)
	frontendTransferId := ctx.Query("transfer_id")
	targetPath := ctx.DefaultQuery("path", "/")

	// Use frontend provided transfer_id or generate new one
	var transferId string
	if frontendTransferId != "" {
		transferId = frontendTransferId
	} else {
		transferId = fmt.Sprintf("rdp-%s-%d", sessionId, time.Now().UnixNano())
	}

	// Create progress record IMMEDIATELY when request starts
	service.CreateTransferProgress(transferId, "rdp")

	tunnel, err := c.validateRDPAccess(ctx, sessionId)
	if err != nil {
		if strings.Contains(err.Error(), "permission") {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: err.Error(),
			})
		} else {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
		}
		return
	}

	if !service.IsRDPDriveEnabled(tunnel) {
		logger.L().Error("RDP drive is not enabled for session", zap.String("sessionId", sessionId))
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "RDP drive is not enabled for this session",
		})
		return
	}

	if !service.IsRDPUploadAllowed(tunnel) {
		logger.L().Error("RDP upload is disabled for session", zap.String("sessionId", sessionId))
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "File upload is disabled for this session",
		})
		return
	}

	// Parse multipart form with streaming
	contentType := ctx.GetHeader("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid content type, expected multipart/form-data",
		})
		return
	}

	// Get boundary from content type
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid content type: %v", err),
		})
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Missing boundary in content type",
		})
		return
	}

	// Create multipart reader for streaming
	reader := multipart.NewReader(ctx.Request.Body, boundary)

	var filename string
	var fileSize int64

	// Find the file part and save to temporary file (avoid memory overhead)
	var tempFilePath string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			ctx.JSON(http.StatusBadRequest, HttpResponse{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Error reading multipart: %v", err),
			})
			return
		}

		formName := part.FormName()
		if formName == "file" {
			filename = part.FileName()
			if filename == "" {
				part.Close()
				continue
			}

			// Create temporary file to store upload data (streaming, no memory overhead)
			tempDir := filepath.Join(os.TempDir(), "oneterm-rdp-uploads")
			if err := os.MkdirAll(tempDir, 0755); err != nil {
				part.Close()
				ctx.JSON(http.StatusInternalServerError, HttpResponse{
					Code:    http.StatusInternalServerError,
					Message: fmt.Sprintf("Failed to create temp directory: %v", err),
				})
				return
			}

			tempFile, err := os.CreateTemp(tempDir, fmt.Sprintf("rdp_upload_%s_*", sessionId))
			if err != nil {
				part.Close()
				ctx.JSON(http.StatusInternalServerError, HttpResponse{
					Code:    http.StatusInternalServerError,
					Message: fmt.Sprintf("Failed to create temp file: %v", err),
				})
				return
			}
			tempFilePath = tempFile.Name()

			// Stream file data directly to temp file (memory-efficient)
			written, err := io.Copy(tempFile, part)
			tempFile.Close()
			part.Close()

			if err != nil {
				os.Remove(tempFilePath)
				ctx.JSON(http.StatusInternalServerError, HttpResponse{
					Code:    http.StatusInternalServerError,
					Message: fmt.Sprintf("Failed to save file data: %v", err),
				})
				return
			}

			fileSize = written
			break // Found and saved the file
		} else {
			part.Close()
		}
	}

	if filename == "" || tempFilePath == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "No file found in upload",
		})
		return
	}

	// Use default path if not provided
	if targetPath == "" {
		targetPath = "/"
	}

	fullPath := filepath.Join(targetPath, filename)

	// Update progress record with file size
	service.UpdateTransferProgress(transferId, fileSize, 0, "transferring")

	// Open temp file for reading and upload synchronously
	tempFile, err := os.Open(tempFilePath)
	if err != nil {
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to open temp file for RDP upload: %v", err),
		})
		return
	}
	defer tempFile.Close()

	// Perform RDP upload synchronously
	err = service.UploadRDPFileStreamWithID(tunnel, transferId, sessionId, fullPath, tempFile, fileSize)
	if err != nil {
		service.UpdateTransferProgress(transferId, 0, -1, "failed")
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to upload file to RDP session: %v", err),
		})
		return
	}

	// Clean up temp file after successful upload
	os.Remove(tempFilePath)

	// Record file history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "upload", fullPath); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Return success response after upload completion
	responseData := gin.H{
		"message": "File uploaded successfully",
		"path":    fullPath,
		"size":    fileSize,
		"status":  "completed",
	}

	if transferId != "" {
		responseData["transfer_id"] = transferId
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "Upload completed",
		Data:    responseData,
	})

	// Clean up progress record after a short delay
	go func() {
		time.Sleep(30 * time.Second) // Keep for 30 seconds for any delayed queries
		service.CleanupTransferProgress(transferId, 0)
	}()
}

// RDPFileDownload downloads files from RDP session drive
// @Summary Download files from RDP session
// @Description Download files from RDP session drive (supports multiple files via names parameter)
// @Tags RDP File
// @Accept json
// @Produce application/octet-stream
// @Param session_id path string true "Session ID"
// @Param dir query string true "Directory path"
// @Param names query string true "File names (comma-separated for multiple files)"
// @Success 200 {file} binary
// @Router /rdp/sessions/{session_id}/files/download [get]
func (c *Controller) RDPFileDownload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	tunnel, validationErr := c.validateRDPAccess(ctx, sessionId)
	if validationErr != nil {
		if strings.Contains(validationErr.Error(), "permission") {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: validationErr.Error(),
			})
		} else {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: validationErr.Error(),
			})
		}
		return
	}

	if !service.IsRDPDriveEnabled(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "Drive redirection not enabled",
		})
		return
	}

	if !service.IsRDPDownloadAllowed(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "File download not allowed",
		})
		return
	}

	// Parse query parameters
	dir := ctx.Query("dir")
	if dir == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Directory parameter is required",
		})
		return
	}

	filenameParam := ctx.Query("names")
	if filenameParam == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Filenames parameter is required",
		})
		return
	}

	// Parse and validate filenames
	filenames := lo.Filter(
		lo.Map(strings.Split(filenameParam, ","), func(name string, _ int) string {
			return strings.TrimSpace(name)
		}),
		func(name string, _ int) bool {
			return name != ""
		},
	)

	if len(filenames) == 0 {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "No valid filenames provided",
		})
		return
	}

	var reader io.ReadCloser
	var downloadFilename string
	var fileSize int64
	var err error

	if len(filenames) == 1 {
		// Single file download (memory-efficient streaming)
		path := filepath.Join(dir, filenames[0])
		reader, fileSize, err = service.DownloadRDPFile(tunnel, path)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, HttpResponse{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to download file: %v", err),
			})
			return
		}

		downloadFilename = filenames[0]
	} else {
		// Multiple files download as ZIP
		reader, downloadFilename, fileSize, err = service.DownloadRDPMultiple(tunnel, dir, filenames)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, HttpResponse{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to download files: %v", err),
			})
			return
		}
	}
	defer reader.Close()

	// Record file operation history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "download", filepath.Join(dir, strings.Join(filenames, ","))); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Set response headers for file download
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", downloadFilename))
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")

	// Set content length if known
	if fileSize > 0 {
		ctx.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	}

	// Stream file content directly to response
	ctx.Status(http.StatusOK)

	// Use streaming copy with buffer to handle large files efficiently
	buffer := make([]byte, 32*1024) // 32KB buffer for optimal performance
	_, err = io.CopyBuffer(ctx.Writer, reader, buffer)
	if err != nil {
		logger.L().Error("File transfer failed", zap.Error(err))
	}
}

// RDPFileMkdir creates directory in RDP session drive
// @Summary Create directory in RDP session
// @Description Create directory in RDP session drive
// @Tags RDP File
// @Accept json
// @Produce json
// @Param session_id path string true "Session ID"
// @Param request body service.RDPMkdirRequest true "Directory creation request"
// @Success 200 {object} HttpResponse
// @Router /rdp/sessions/{session_id}/files/mkdir [post]
func (c *Controller) RDPFileMkdir(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	var req service.RDPMkdirRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request parameters",
		})
		return
	}

	tunnel, validateErr := c.validateRDPAccess(ctx, sessionId)
	if validateErr != nil {
		if strings.Contains(validateErr.Error(), "permission") {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: validateErr.Error(),
			})
		} else {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: validateErr.Error(),
			})
		}
		return
	}

	// Check if upload is allowed (mkdir is considered an upload operation)
	if !service.IsRDPUploadAllowed(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "Directory creation is disabled for this session",
		})
		return
	}

	// Send mkdir request through Guacamole protocol
	err := service.CreateRDPDirectory(tunnel, req.Path)
	if err != nil {
		logger.L().Error("Failed to create directory in RDP session", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create directory",
		})
		return
	}

	// Record file operation history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "mkdir", req.Path); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "ok",
		Data: gin.H{
			"message": "Directory created successfully",
			"path":    req.Path,
		},
	})
}

func (c *Controller) validateRDPAccess(ctx *gin.Context, sessionId string) (*guacd.Tunnel, error) {
	currentUser, err := acl.GetSessionFromCtx(ctx)
	if err != nil || currentUser == nil {
		return nil, fmt.Errorf("no permission to access this session")
	}

	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		return nil, fmt.Errorf("session not found or not active")
	}

	tunnel := onlineSession.GuacdTunnel
	if tunnel == nil {
		return nil, fmt.Errorf("session not found or not active")
	}

	return tunnel, nil
}

// =============================================================================
// Sftp-based File Operations
// =============================================================================

// SftpFileLS godoc
//
//	@Tags		file
//	@Param		session_id		path		string	true	"session_id"
//	@Param		dir				query		string	true	"dir"
//	@Param		show_hidden		query		bool	false	"show hidden files (default: false)"
//	@Success	200				{object}	HttpResponse
//	@Router		/file/session/:session_id/ls [GET]
func (c *Controller) SftpFileLS(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	dir := ctx.Query("dir")

	if dir == "" {
		dir = "/"
	}

	// Check if session is active
	if !service.DefaultFileService.IsSessionActive(sessionId) {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or inactive",
		})
		return
	}

	// Get session info for authorization check
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found",
		})
		return
	}

	// Check authorization using the same logic as legacy API
	if ok, err := hasAuthorization(ctx, onlineSession); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use session-based file service
	fileInfos, err := service.DefaultFileService.SessionLS(ctx, sessionId, dir)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: "Session not found",
			})
		} else if service.IsPermissionError(err) {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, HttpResponse{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to list directory: %v", err),
			})
		}
		return
	}

	// Filter hidden files unless show_hidden is true
	showHidden := cast.ToBool(ctx.Query("show_hidden"))
	if !showHidden {
		var filtered []service.FileInfo
		for _, f := range fileInfos {
			if !strings.HasPrefix(f.Name, ".") {
				filtered = append(filtered, f)
			}
		}
		fileInfos = filtered
	}

	res := &ListData{
		Count: int64(len(fileInfos)),
		List:  lo.Map(fileInfos, func(f service.FileInfo, _ int) any { return f }),
	}
	ctx.JSON(http.StatusOK, NewHttpResponseWithData(res))
}

// SftpFileMkdir godoc
//
//	@Tags		file
//	@Param		session_id		path		string	true	"session_id"
//	@Param		dir				query		string	true	"dir"
//	@Success	200				{object}	HttpResponse
//	@Router		/file/session/:session_id/mkdir [post]
func (c *Controller) SftpFileMkdir(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	dir := ctx.Query("dir")

	if dir == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Directory path is required",
		})
		return
	}

	// Check if session is active
	if !service.DefaultFileService.IsSessionActive(sessionId) {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or inactive",
		})
		return
	}

	// Get session info for authorization check
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found",
		})
		return
	}

	// Check authorization using the same logic as legacy API
	if ok, err := hasAuthorization(ctx, onlineSession); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use session-based file service
	if err := service.DefaultFileService.SessionMkdir(ctx, sessionId, dir); err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: "Session not found",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, HttpResponse{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to create directory: %v", err),
			})
		}
		return
	}

	// Record history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "mkdir", dir); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// TransferProgressById - Unified transfer progress tracking for SFTP and RDP
// @Tags file
// @Router /file/transfer/progress/id/:transfer_id [get]
func (c *Controller) TransferProgressById(ctx *gin.Context) {
	transferId := ctx.Param("transfer_id")

	// First check unified progress tracking
	progress, exists := service.GetTransferProgressById(transferId)

	if exists {
		// Calculate transfer progress
		var progressPercent int
		if progress.TotalSize > 0 {
			progressPercent = int(float64(progress.TransferredSize) / float64(progress.TotalSize) * 100)
		}

		ctx.JSON(http.StatusOK, HttpResponse{
			Code:    0,
			Message: "ok",
			Data: gin.H{
				"status":   progress.Status,
				"progress": progressPercent,
				"type":     progress.Type,
				"message":  fmt.Sprintf("Transferred %d/%d bytes via %s", progress.TransferredSize, progress.TotalSize, strings.ToUpper(progress.Type)),
			},
		})
		return
	}

	// Fallback: check RDP guacd transfer manager
	rdpProgress, err := service.GetRDPTransferProgressById(transferId)
	if err == nil {
		ctx.JSON(http.StatusOK, HttpResponse{
			Code:    0,
			Message: "ok",
			Data:    rdpProgress,
		})
		return
	}

	// Transfer not found
	ctx.JSON(http.StatusNotFound, HttpResponse{
		Code:    http.StatusNotFound,
		Message: "Transfer not found or already completed",
		Data: gin.H{
			"status":   "not_found",
			"progress": 0,
			"message":  "Transfer not found in progress tracking",
		},
	})
}

// Helper methods for RDP transfer progress

// RDPFileTransferPrepare creates transfer records before upload starts
// @Summary Create transfer record for RDP upload
// @Description Create transfer record before RDP upload starts for progress tracking
// @Tags RDP File
// @Accept json
// @Produce json
// @Param session_id path string true "Session ID"
// @Param transfer_id query string false "Custom transfer ID"
// @Param filename query string false "Filename"
// @Success 200 {object} HttpResponse
// @Router /rdp/sessions/{session_id}/files/prepare [post]
func (c *Controller) RDPFileTransferPrepare(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	transferId := ctx.Query("transfer_id")
	filename := ctx.Query("filename")

	if transferId == "" {
		transferId = fmt.Sprintf("rdp-%s-%d", sessionId, time.Now().UnixNano())
	}

	// Create unified progress tracking entry
	service.CreateTransferProgress(transferId, "rdp")

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "Transfer prepared",
		Data: gin.H{
			"transfer_id": transferId,
			"status":      "prepared",
			"filename":    filename,
		},
	})
}

// SftpFileUpload godoc
//
//	@Tags		file
//	@Summary	High-performance file upload using optimized SFTP
//	@Description Uploads file via server temp storage then transfers to target using optimized SFTP with performance enhancements. HTTP response only after file reaches target machine.
//	@Param		session_id		path		string	true	"session_id"
//	@Param		dir				query		string	false	"target directory path (default: /tmp)"
//	@Param		transfer_id		query		string	false	"Custom transfer ID for progress tracking (frontend generated)"
//	@Accept		multipart/form-data
//	@Param		file			formData	file	true	"file to upload"
//	@Success	200				{object}	HttpResponse
//	@Router		/file/session/:session_id/upload [post]
func (c *Controller) SftpFileUpload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	// Get transfer_id from URL query parameters IMMEDIATELY (non-blocking)
	frontendTransferId := ctx.Query("transfer_id")
	targetDir := ctx.DefaultQuery("dir", "/tmp")

	// Use frontend provided transfer_id or generate new one
	var transferId string
	if frontendTransferId != "" {
		transferId = frontendTransferId
	} else {
		transferId = fmt.Sprintf("%s-%d", sessionId, time.Now().UnixNano())
	}

	service.CreateTransferProgress(transferId, "sftp")

	// Validate session
	if !service.DefaultFileService.IsSessionActive(sessionId) {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or inactive",
		})
		return
	}

	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found",
		})
		return
	}

	if ok, err := hasAuthorization(ctx, onlineSession); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Parse multipart form with memory limit, have many time for big file
	if err := ctx.Request.ParseMultipartForm(MaxMemoryForParsing); err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Failed to parse multipart form: %v", err),
		})
		return
	}

	file, fileHeader, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Failed to get uploaded file: %v", err),
		})
		return
	}
	defer file.Close()

	filename := fileHeader.Filename
	fileSize := fileHeader.Size

	if fileSize > MaxFileSize {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("File size %d bytes exceeds limit of %d bytes", fileSize, MaxFileSize),
		})
		return
	}

	// Update transfer progress with file size now that we have it
	service.UpdateTransferProgress(transferId, fileSize, 0, "")

	// Phase 1: Save file to server temp directory
	tempDir := filepath.Join(os.TempDir(), "oneterm-uploads", sessionId)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to create temp directory: %v", err),
		})
		return
	}

	tempFilePath := filepath.Join(tempDir, filename)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to create temp file: %v", err),
		})
		return
	}

	// Copy uploaded file to temp location
	written, err := io.Copy(tempFile, file)
	tempFile.Close()

	if err != nil {
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("Failed to save file: %v", err),
		})
		return
	}

	if written != fileSize {
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("File size mismatch: expected %d, got %d", fileSize, written),
		})
		return
	}

	targetPath := filepath.Join(targetDir, filename)

	// Phase 2: Transfer to target machine using SFTP (synchronous)
	service.UpdateTransferProgress(transferId, fileSize, 0, "transferring")

	if err := service.TransferToTarget(transferId, sessionId, tempFilePath, targetPath, 0, 0); err != nil {
		// Mark transfer as failed and clean up
		service.UpdateTransferProgress(transferId, 0, -1, "failed")
		os.Remove(tempFilePath)
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: fmt.Sprintf("File transfer failed: %v", err),
		})
		return
	}

	// Mark transfer as completed (success)
	service.UpdateTransferProgress(transferId, 0, -1, "completed")

	// Clean up temp file after successful transfer
	os.Remove(tempFilePath)

	// Record file history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "upload", filepath.Join(targetDir, filename)); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Return success response after transfer completion
	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "File uploaded successfully",
		Data: gin.H{
			"filename":    filename,
			"path":        targetPath,
			"size":        fileSize,
			"transfer_id": transferId,
			"status":      "completed",
		},
	})

	// Clean up progress record after a short delay
	go func() {
		time.Sleep(30 * time.Second) // Keep for 30 seconds for any delayed queries
		service.CleanupTransferProgress(transferId, 0)
	}()
}

// SftpFileDownload godoc
//
//	@Tags		file
//	@Param		session_id		path		string	true	"session_id"
//	@Param		dir				query		string	true	"dir"
//	@Param		names			query		string	true	"names (comma-separated for multiple files)"
//	@Success	200				{object}	HttpResponse
//	@Router		/file/session/:session_id/download [get]
func (c *Controller) SftpFileDownload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	// Check if session is active
	if !service.DefaultFileService.IsSessionActive(sessionId) {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or inactive",
		})
		return
	}

	// Get session info for authorization check
	onlineSession := gsession.GetOnlineSessionById(sessionId)
	if onlineSession == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found",
		})
		return
	}

	// Check authorization using the same logic as legacy API
	if ok, err := hasAuthorization(ctx, onlineSession); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &myErrors.ApiError{Code: myErrors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &myErrors.ApiError{Code: myErrors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	filenameParam := ctx.Query("names")
	if filenameParam == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "names parameter is required",
		})
		return
	}

	filenames := lo.Filter(
		lo.Map(strings.Split(filenameParam, ","), func(name string, _ int) string {
			return strings.TrimSpace(name)
		}),
		func(name string, _ int) bool {
			return name != ""
		},
	)

	if len(filenames) == 0 {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "No valid filenames provided",
		})
		return
	}

	reader, downloadFilename, fileSize, err := service.DefaultFileService.SessionDownloadMultiple(ctx, sessionId, ctx.Query("dir"), filenames)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			ctx.JSON(http.StatusNotFound, HttpResponse{
				Code:    http.StatusNotFound,
				Message: "Session not found",
			})
		} else if service.IsPermissionError(err) {
			ctx.JSON(http.StatusForbidden, HttpResponse{
				Code:    http.StatusForbidden,
				Message: "Permission denied",
			})
		} else {
			ctx.JSON(http.StatusInternalServerError, HttpResponse{
				Code:    http.StatusInternalServerError,
				Message: fmt.Sprintf("Failed to download files: %v", err),
			})
		}
		return
	}
	defer reader.Close()

	// Record file operation history using session-based method
	if err := service.DefaultFileService.RecordFileHistoryBySession(ctx, sessionId, "download", filepath.Join(ctx.Query("dir"), strings.Join(filenames, ","))); err != nil {
		logger.L().Error("Failed to record file history", zap.Error(err))
	}

	// Set response headers for file download
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", downloadFilename))
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")

	// Set content length if known
	if fileSize > 0 {
		ctx.Header("Content-Length", fmt.Sprintf("%d", fileSize))
	}

	// Stream file content directly to response
	ctx.Status(http.StatusOK)

	// Use streaming copy with buffer to handle large files efficiently
	buffer := make([]byte, 32*1024) // 32KB buffer for optimal performance
	_, err = io.CopyBuffer(ctx.Writer, reader, buffer)
	if err != nil {
		logger.L().Error("File transfer failed", zap.Error(err))
	}
}
