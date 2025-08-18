package middleware

import (
	"strings"
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

		// Skip logging for web proxy requests to reduce noise
		url := ctx.Request.URL.String()
		host := ctx.Request.Host
		if strings.HasPrefix(host, "asset-") {
			return
		}

		// Skip logging WebSocket connections to avoid hijacked connection issues
		if _, isWebSocket := ctx.Get("websocket_connection"); isWebSocket {
			logger.L().Debug(url,
				zap.String("method", ctx.Request.Method),
				zap.Int("status", 200),
				zap.String("ip", ctx.ClientIP()),
				zap.Duration("cost", cost),
			)
			return
		}

		// Only log errors and slow requests
		status := ctx.Writer.Status()
		if status >= 400 || cost > 1*time.Second {
			logger.L().Info(url,
				zap.String("method", ctx.Request.Method),
				zap.Int("status", status),
				zap.String("ip", ctx.ClientIP()),
				zap.Duration("cost", cost),
			)
		} else {
			// Normal requests use debug level to reduce log noise
			logger.L().Debug(url,
				zap.String("method", ctx.Request.Method),
				zap.Int("status", status),
				zap.String("ip", ctx.ClientIP()),
				zap.Duration("cost", cost),
			)
		}

	}
}
