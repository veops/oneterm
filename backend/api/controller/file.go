package controller

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/file"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	gsession "github.com/veops/oneterm/session"
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
	db := mysql.DB.Model(&model.FileHistory{})
	currentUser, _ := acl.GetSessionFromCtx(ctx)
	if !acl.IsAdmin(currentUser) {
		db = db.Where("uid = ?", currentUser.Uid)
	}
	db = filterSearch(ctx, db, "user_name")
	db, err := filterStartEnd(ctx, db)
	if err != nil {
		return
	}
	db = filterEqual(ctx, db, "status", "uid", "asset_id", "account_id", "client_ip")

	doGet[*model.FileHistory](ctx, false, db, "")
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

	cli, err := file.GetFileManager().GetFileClient(cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}
	info, err := cli.ReadDir(ctx.Query("dir"))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	res := &ListData{
		Count: int64(len(info)),
		List: lo.Map(info, func(f fs.FileInfo, _ int) any {
			return &file.FileInfo{
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

	cli, err := file.GetFileManager().GetFileClient(cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{}})
		return
	}

	if err = cli.MkdirAll(ctx.Query("dir")); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   cast.ToInt(ctx.Param("asset_id")),
		AccountId: cast.ToInt(ctx.Param("account_id")),
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_MKDIR,
		Dir:       ctx.Query("dir"),
	}
	if err = mysql.DB.Model(h).Create(h).Error; err != nil {
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

	cli, err := file.GetFileManager().GetFileClient(cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{}})
		return
	}
	rf, err := cli.Create(filepath.Join(ctx.Query("dir"), fh.Filename))
	if err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}
	if _, err = rf.Write(content); err != nil {
		ctx.AbortWithError(http.StatusBadRequest, &ApiError{Code: ErrInternal, Data: map[string]any{"err": err}})
		return
	}

	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   cast.ToInt(ctx.Param("asset_id")),
		AccountId: cast.ToInt(ctx.Param("account_id")),
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_UPLOAD,
		Dir:       ctx.Query("dir"),
		Filename:  fh.Filename,
	}
	if err = mysql.DB.Model(h).Create(h).Error; err != nil {
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

	cli, err := file.GetFileManager().GetFileClient(cast.ToInt(ctx.Param("asset_id")), cast.ToInt(ctx.Param("account_id")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInternal, Data: map[string]any{}})
		return
	}
	rf, err := cli.Open(filepath.Join(ctx.Query("dir"), ctx.Query("filename")))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, &ApiError{Code: ErrInvalidArgument, Data: map[string]any{"err": err}})
		return
	}

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", ctx.Query("filename")))
	ctx.Header("Content-Type", "application/text/plain")
	buf := &bytes.Buffer{}
	rf.WriteTo(buf)
	ctx.Header("Accept-Length", fmt.Sprintf("%d", len(buf.Bytes())))
	ctx.Writer.Write(buf.Bytes())

	h := &model.FileHistory{
		Uid:       currentUser.GetUid(),
		UserName:  currentUser.GetUserName(),
		AssetId:   cast.ToInt(ctx.Param("asset_id")),
		AccountId: cast.ToInt(ctx.Param("account_id")),
		ClientIp:  ctx.ClientIP(),
		Action:    model.FILE_ACTION_UPLOAD,
		Dir:       ctx.Query("dir"),
		Filename:  ctx.Query("filename"),
	}

	if err = mysql.DB.Model(h).Create(h).Error; err != nil {
		logger.L().Error("record download failed", zap.Error(err), zap.Any("history", h))
	}
}
