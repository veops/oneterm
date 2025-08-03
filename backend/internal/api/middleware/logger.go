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
		// Only log errors and slow requests
		status := ctx.Writer.Status()
		if status >= 400 || cost > 1*time.Second {
			logger.L().Info(ctx.Request.URL.String(),
				zap.String("method", ctx.Request.Method),
				zap.Int("status", status),
				zap.String("ip", ctx.ClientIP()),
				zap.Duration("cost", cost),
			)
		} else {
			// Normal requests use debug level to reduce log noise
			logger.L().Debug(ctx.Request.URL.String(),
				zap.String("method", ctx.Request.Method),
				zap.Int("status", status),
				zap.String("ip", ctx.ClientIP()),
				zap.Duration("cost", cost),
			)
		}

	}
}
