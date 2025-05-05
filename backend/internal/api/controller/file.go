package controller

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
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
//	@Param		accout_id	query		int		false	"account id"
//	@Param		client_ip	query		string	false	"client_ip"
//	@Success	200			{object}	HttpResponse{data=ListData{list=[]model.Session}}
//	@Router		/file/history [get]
func (c *Controller) GetFileHistory(ctx *gin.Context) {
	// Create filter conditions
	filters := make(map[string]interface{})

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
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
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
//	@Success	200			{object}	HttpResponse
//	@Router		/file/ls/:asset_id/:account_id [post]
func (c *Controller) FileLS(ctx *gin.Context) {
	sess := &gsession.Session{
		Session: &model.Session{
			AssetId:   cast.ToInt(ctx.Param("asset_id")),
			AccountId: cast.ToInt(ctx.Param("account_id")),
		},
	}

	if !hasAuthorization(ctx, sess) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	info, err := service.DefaultFileService.ReadDir(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	res := &ListData{
		Count: int64(len(info)),
		List: lo.Map(info, func(f fs.FileInfo, _ int) any {
			return &service.FileInfo{
				Name:  f.Name(),
				IsDir: f.IsDir(),
				Size:  f.Size(),
				Mode:  f.Mode().String(),
			}
		}),
	}
	ctx.JSON(http.StatusOK, NewHttpResponseWithData(res))
}

// FileMkdir file
//
//	@Tags		account
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

	if !hasAuthorization(ctx, sess) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	if err := service.DefaultFileService.MkdirAll(ctx, sess.Session.AssetId, sess.Session.AccountId, ctx.Query("dir")); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
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

	if !hasAuthorization(ctx, sess) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{}})
		return
	}

	f, fh, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	content, err := io.ReadAll(f)
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	// Use global file service
	rf, err := service.DefaultFileService.Create(ctx, sess.Session.AssetId, sess.Session.AccountId, filepath.Join(ctx.Query("dir"), fh.Filename))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	if _, err = rf.Write(content); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
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
//	@Param		filename	query		string	true	"filename"
//	@Param		file		formData	string	true	"file field name"
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

	if !hasAuthorization(ctx, sess) {
		ctx.AbortWithError(http.StatusForbidden, &ApiError{Code: ErrNoPerm, Data: map[string]any{}})
		return
	}

	// Use global file service
	rf, err := service.DefaultFileService.Open(ctx, sess.Session.AssetId, sess.Session.AccountId, filepath.Join(ctx.Query("dir"), ctx.Query("filename")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	defer rf.Close()

	content, err := io.ReadAll(rf)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	// Create history record
	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   sess.Session.AssetId,
		AccountId: sess.Session.AccountId,
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_DOWNLOAD,
		Dir:       ctx.Query("dir"),
		Filename:  ctx.Query("filename"),
	}

	if err = service.DefaultFileService.AddFileHistory(ctx, h); err != nil {
		logger.L().Error("record download failed", zap.Error(err), zap.Any("history", h))
	}

	rw := ctx.Writer
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", ctx.Query("filename")))
	rw.Header().Set("Content-Type", "application/octet-stream")
	rw.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	rw.Write(content)
}
