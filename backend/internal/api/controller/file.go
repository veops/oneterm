package controller

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

func isPermissionError(err error) bool {
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
	// Create filter conditions
	filters := make(map[string]any)

	// Get user permissions
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		filters["uid"] = currentUser.Uid
	}

	// Add other filter conditions
	if search := ctx.Query("search"); search != "" {
		filters["user_name LIKE ?"] = "%" + search + "%"
	}

	if status := ctx.Query("status"); status != "" {
		filters["status"] = cast.ToInt(status)
	}

	if uid := ctx.Query("uid"); uid != "" {
		filters["uid"] = cast.ToInt(uid)
	}

	if assetId := ctx.Query("asset_id"); assetId != "" {
		filters["asset_id"] = cast.ToInt(assetId)
	}

	if accountId := ctx.Query("account_id"); accountId != "" {
		filters["account_id"] = cast.ToInt(accountId)
	}

	if clientIp := ctx.Query("client_ip"); clientIp != "" {
		filters["client_ip"] = clientIp
	}

	if action := ctx.Query("action"); action != "" {
		filters["action"] = cast.ToInt(action)
	}

	// Process time range
	if start := ctx.Query("start"); start != "" {
		filters["created_at >= ?"] = start
	}

	if end := ctx.Query("end"); end != "" {
		filters["created_at <= ?"] = end
	}

	// Use global file service
	histories, count, err := service.DefaultFileService.GetFileHistory(ctx, filters)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Convert to slice of any type
	historiesAny := make([]any, 0, len(histories))
	for _, h := range histories {
		historiesAny = append(historiesAny, h)
	}

	result := &ListData{
		Count: count,
		List:  historiesAny,
	}

	ctx.JSON(http.StatusOK, HttpResponse{
		Data: result,
	})
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
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &errors.ApiError{Code: errors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	info, err := service.DefaultFileService.ReadDir(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir"))
	if err != nil {
		if isPermissionError(err) {
			ctx.AbortWithError(http.StatusForbidden, fmt.Errorf("permission denied"))
		} else {
			ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
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
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &errors.ApiError{Code: errors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	if err := service.DefaultFileService.MkdirAll(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir")); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Create history record
	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   sess.Session.AssetId,
		AccountId: sess.Session.AccountId,
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_MKDIR,
		Dir:       ctx.Query("dir"),
	}

	if err := service.DefaultFileService.AddFileHistory(ctx, h); err != nil {
		logger.L().Error("record mkdir failed", zap.Error(err), zap.Any("history", h))
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
}

// FileUpload godoc
//
//	@Tags		file
//	@Param		asset_id	path		int		true	"asset_id"
//	@Param		account_id	path		int		true	"account_id"
//	@Param		path		query		string	true	"path"
//	@Success	200			{object}	HttpResponse
//	@Router		/file/upload/:asset_id/:account_id [post]
func (c *Controller) FileUpload(ctx *gin.Context) {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &errors.ApiError{Code: errors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	f, fh, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	content, err := io.ReadAll(f)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Use global file service
	rf, err := service.DefaultFileService.Create(ctx, sess.Session.AssetId, sess.Session.AccountId, filepath.Join(ctx.Query("dir"), fh.Filename))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	if _, err = rf.Write(content); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Create history record
	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   sess.Session.AssetId,
		AccountId: sess.Session.AccountId,
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_UPLOAD,
		Dir:       ctx.Query("dir"),
		Filename:  fh.Filename,
	}

	if err = service.DefaultFileService.AddFileHistory(ctx, h); err != nil {
		logger.L().Error("record upload failed", zap.Error(err), zap.Any("history", h))
	}

	ctx.JSON(http.StatusOK, defaultHttpResponse)
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
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if ok, err := hasAuthorization(ctx, sess); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	} else if !ok {
		ctx.AbortWithError(http.StatusForbidden, &errors.ApiError{Code: errors.ErrNoPerm, Data: map[string]any{}})
		return
	}

	filenameParam := ctx.Query("names")
	if filenameParam == "" {
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": "names parameter is required"}})
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
		ctx.AbortWithError(http.StatusBadRequest, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": "no valid filenames provided"}})
		return
	}

	reader, downloadFilename, fileSize, err := service.DefaultFileService.DownloadMultiple(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir"), filenames)
	if err != nil {
		if isPermissionError(err) {
			ctx.AbortWithError(http.StatusForbidden, &errors.ApiError{Code: errors.ErrNoPerm, Data: map[string]any{"err": err}})
		} else {
			ctx.AbortWithError(http.StatusInternalServerError, &errors.ApiError{Code: errors.ErrInvalidArgument, Data: map[string]any{"err": err}})
		}
		return
	}
	defer reader.Close()

	// Record file operation history
	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   sess.Session.AssetId,
		AccountId: sess.Session.AccountId,
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_DOWNLOAD,
		Dir:       ctx.Query("dir"),
		Filename:  strings.Join(filenames, ","),
		CreatedAt: time.Now(),
	}
	service.DefaultFileService.AddFileHistory(ctx, h)

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

// RDPFileInfo represents file information for RDP sessions
type RDPFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// RDPMkdirRequest represents directory creation request for RDP
type RDPMkdirRequest struct {
	Path string `json:"path" binding:"required"`
}

// RDPFileList lists files in RDP session drive
// @Summary List RDP session files
// @Description Get file list for RDP session drive
// @Tags RDP File
// @Param session_id path string true "Session ID"
// @Param path query string false "Directory path"
// @Success	200			{object}	HttpResponse
// @Router /api/v1/rdp/sessions/{session_id}/files [get]
func (c *Controller) RDPFileList(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	path := ctx.DefaultQuery("path", "/")

	// Check if session exists and user has permission
	if !c.hasRDPSessionPermission(ctx, sessionId) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "No permission to access this session",
		})
		return
	}

	// Get session tunnel
	tunnel := c.getRDPSessionTunnel(sessionId)
	if tunnel == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or not active",
		})
		return
	}

	// Check if RDP drive is enabled
	if !c.isRDPDriveEnabled(tunnel) {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "RDP drive is not enabled for this session",
		})
		return
	}

	// Send file list request through Guacamole protocol
	files, err := c.requestRDPFileList(tunnel, path)
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
// @Param file formData file true "File to upload"
// @Param path formData string false "Target directory path"
// @Success 200 {object} HttpResponse
// @Router /api/v1/rdp/sessions/{session_id}/files/upload [post]
func (c *Controller) RDPFileUpload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	targetPath := ctx.DefaultPostForm("path", "/")

	// Check permission
	if !c.hasRDPSessionPermission(ctx, sessionId) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "No permission to access this session",
		})
		return
	}

	// Get uploaded file
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Failed to get uploaded file",
		})
		return
	}
	defer file.Close()

	// Get session tunnel
	tunnel := c.getRDPSessionTunnel(sessionId)
	if tunnel == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or not active",
		})
		return
	}

	// Check if upload is allowed
	if !c.isRDPUploadAllowed(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "File upload is disabled for this session",
		})
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to read file content",
		})
		return
	}

	// Send upload request through Guacamole protocol
	fullPath := filepath.Join(targetPath, header.Filename)
	err = c.uploadRDPFile(tunnel, fullPath, content)
	if err != nil {
		logger.L().Error("Failed to upload file to RDP session", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to upload file",
		})
		return
	}

	// Record file operation history
	c.recordRDPFileHistory(ctx, sessionId, "upload", fullPath, int64(len(content)))

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "ok",
		Data: gin.H{
			"message": "File uploaded successfully",
			"path":    fullPath,
			"size":    len(content),
		},
	})
}

// RDPFileDownload downloads file from RDP session drive
// @Summary Download file from RDP session
// @Description Download file from RDP session drive
// @Tags RDP File
// @Accept json
// @Produce application/octet-stream
// @Param session_id path string true "Session ID"
// @Param path query string true "File path"
// @Success 200 {file} binary
// @Router /api/v1/rdp/sessions/{session_id}/files/download [get]
func (c *Controller) RDPFileDownload(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")
	filePath := ctx.Query("path")

	if filePath == "" {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "File path is required",
		})
		return
	}

	// Check permission
	if !c.hasRDPSessionPermission(ctx, sessionId) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "No permission to access this session",
		})
		return
	}

	// Get session tunnel
	tunnel := c.getRDPSessionTunnel(sessionId)
	if tunnel == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or not active",
		})
		return
	}

	// Check if download is allowed
	if !c.isRDPDownloadAllowed(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "File download is disabled for this session",
		})
		return
	}

	// Request file download through Guacamole protocol
	content, err := c.downloadRDPFile(tunnel, filePath)
	if err != nil {
		logger.L().Error("Failed to download file from RDP session", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to download file",
		})
		return
	}

	// Record file operation history
	c.recordRDPFileHistory(ctx, sessionId, "download", filePath, int64(len(content)))

	// Set response headers
	filename := filepath.Base(filePath)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Length", strconv.Itoa(len(content)))

	ctx.Data(http.StatusOK, "application/octet-stream", content)
}

// RDPFileMkdir creates directory in RDP session drive
// @Summary Create directory in RDP session
// @Description Create directory in RDP session drive
// @Tags RDP File
// @Accept json
// @Produce json
// @Param session_id path string true "Session ID"
// @Param request body RDPMkdirRequest true "Directory creation request"
// @Success 200 {object} HttpResponse
// @Router /api/v1/rdp/sessions/{session_id}/files/mkdir [post]
func (c *Controller) RDPFileMkdir(ctx *gin.Context) {
	sessionId := ctx.Param("session_id")

	var req RDPMkdirRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request parameters",
		})
		return
	}

	// Check permission
	if !c.hasRDPSessionPermission(ctx, sessionId) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "No permission to access this session",
		})
		return
	}

	// Get session tunnel
	tunnel := c.getRDPSessionTunnel(sessionId)
	if tunnel == nil {
		ctx.JSON(http.StatusNotFound, HttpResponse{
			Code:    http.StatusNotFound,
			Message: "Session not found or not active",
		})
		return
	}

	// Check if upload is allowed (mkdir is considered an upload operation)
	if !c.isRDPUploadAllowed(tunnel) {
		ctx.JSON(http.StatusForbidden, HttpResponse{
			Code:    http.StatusForbidden,
			Message: "Directory creation is disabled for this session",
		})
		return
	}

	// Send mkdir request through Guacamole protocol
	err := c.createRDPDirectory(tunnel, req.Path)
	if err != nil {
		logger.L().Error("Failed to create directory in RDP session", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to create directory",
		})
		return
	}

	// Record file operation history
	c.recordRDPFileHistory(ctx, sessionId, "mkdir", req.Path, 0)

	ctx.JSON(http.StatusOK, HttpResponse{
		Code:    0,
		Message: "ok",
		Data: gin.H{
			"message": "Directory created successfully",
			"path":    req.Path,
		},
	})
}

// RDP Helper methods

func (c *Controller) hasRDPSessionPermission(ctx *gin.Context, sessionId string) bool {
	// TODO: Implement proper session permission check
	// This should verify that the current user has access to the specified session
	return true
}

func (c *Controller) getRDPSessionTunnel(sessionId string) *guacd.Tunnel {
	// TODO: Implement session tunnel retrieval
	// This should get the active Guacamole tunnel for the session
	return nil
}

func (c *Controller) isRDPDriveEnabled(tunnel *guacd.Tunnel) bool {
	// Check if RDP drive is enabled in tunnel configuration
	return tunnel.Config.Parameters["enable-drive"] == "true"
}

func (c *Controller) isRDPUploadAllowed(tunnel *guacd.Tunnel) bool {
	return tunnel.Config.Parameters["disable-upload"] != "true"
}

func (c *Controller) isRDPDownloadAllowed(tunnel *guacd.Tunnel) bool {
	return tunnel.Config.Parameters["disable-download"] != "true"
}

func (c *Controller) requestRDPFileList(tunnel *guacd.Tunnel, path string) ([]RDPFileInfo, error) {
	// TODO: Implement Guacamole protocol communication for file listing
	// This would involve sending appropriate Guacamole instructions and parsing responses
	return nil, fmt.Errorf("not implemented: RDP file listing through Guacamole protocol")
}

func (c *Controller) uploadRDPFile(tunnel *guacd.Tunnel, path string, content []byte) error {
	// TODO: Implement Guacamole protocol communication for file upload
	// This would involve sending file-upload instruction with base64 encoded content
	return fmt.Errorf("not implemented: RDP file upload through Guacamole protocol")
}

func (c *Controller) downloadRDPFile(tunnel *guacd.Tunnel, path string) ([]byte, error) {
	// TODO: Implement Guacamole protocol communication for file download
	// This would involve sending file-download instruction and receiving file data
	return nil, fmt.Errorf("not implemented: RDP file download through Guacamole protocol")
}

func (c *Controller) createRDPDirectory(tunnel *guacd.Tunnel, path string) error {
	// TODO: Implement Guacamole protocol communication for directory creation
	return fmt.Errorf("not implemented: RDP directory creation through Guacamole protocol")
}

func (c *Controller) recordRDPFileHistory(ctx *gin.Context, sessionId, operation, path string, size int64) {
	// Record file operation in history
	history := &model.FileHistory{
		Uid:       0,  // TODO: Get current user ID
		UserName:  "", // TODO: Get current user name
		AssetId:   0,  // TODO: Get asset ID from session
		AccountId: 0,  // TODO: Get account ID from session
		ClientIp:  ctx.ClientIP(),
		Action:    c.getRDPActionCode(operation),
		Dir:       filepath.Dir(path),
		Filename:  filepath.Base(path),
	}

	fileService := service.NewFileService(repository.NewFileRepository(dbpkg.DB))
	if err := fileService.AddFileHistory(ctx, history); err != nil {
		logger.L().Error("Failed to record RDP file history", zap.Error(err))
	}
}

func (c *Controller) getRDPActionCode(operation string) int {
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
