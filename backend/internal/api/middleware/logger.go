package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
)

func LoggerMiddleware() gin.HandlerFunc {
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
