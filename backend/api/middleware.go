package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/logger"
)

func ginLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		ctx.Next()

		cost := time.Since(start)
		logger.L().Info(ctx.Request.URL.String(),
			zap.String("method", ctx.Request.Method),
			zap.Int("status", ctx.Writer.Status()),
			zap.String("ip", ctx.ClientIP()),
			zap.Duration("cost", cost),
		)

	}
}

func auth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		cookie, err := ctx.Cookie("session")
		if err != nil || cookie == "" {
			logger.L().Error("cannot get cookie.session", zap.Error(err))
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		sess, err := acl.ParseCookie(cookie)
		if err != nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set("session", sess)

		ctx.Next()
	}
}
