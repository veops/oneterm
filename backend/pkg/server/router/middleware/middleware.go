package middleware

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/pkg/logger"
)

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token, session")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers")
			c.Header("Access-Control-Max-Age", "172800")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		defer func() {
			if err := recover(); err != nil {
				logger.L.Sugar().Errorf("Panic info is: %v", err)
			}
		}()

		c.Next()
	}
}

func RecoveryWithWriter() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				logger.L.Error(string(buf[:n]))
			}
		}()

		c.Next()
	}
}

type responseWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}
