package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	errUnauthorized = &errors.ApiError{Code: errors.ErrUnauthorized}
)

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var (
			sess   *acl.Session
			err    error
			cookie string
		)

		m := make(map[string]any)
		contentType := ctx.GetHeader("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			ctx.ShouldBindBodyWithJSON(&m)
		}

		if ctx.Request.Method == "GET" {
			if _, ok := ctx.GetQuery("_key"); ok {
				m["_key"] = ctx.Query("_key")
				m["_secret"] = ctx.Query("_secret")
			}
		}
		if _, ok := m["_key"]; ok {
			sess, err = acl.AuthWithKey(ctx.Request.URL.Path, m)
			if err != nil {
				logger.L().Error("cannot authwithkey", zap.Error(err))
				ctx.AbortWithError(http.StatusUnauthorized, errUnauthorized)
				return
			}
			ctx.Set("isAuthWithKey", true)
		} else {
			cookie, err = ctx.Cookie("session")
			if err != nil || cookie == "" {
				logger.L().Error("cannot get cookie.session", zap.Error(err))
				ctx.AbortWithError(http.StatusUnauthorized, errUnauthorized)
				return
			}
			sess, err = acl.ParseCookie(cookie)
		}

		if err != nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set("session", sess)
		ctx.Next()
	}
}

func authAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser, _ := acl.GetSessionFromCtx(ctx)
		if !acl.IsAdmin(currentUser) {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}
}
