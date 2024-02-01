// Package middleware
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	handler "github.com/veops/oneterm/pkg/server/controller"
)

var (
	NotLogUrls = []string{"/favicon.ico"}
)

func GinLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		if !lo.Contains(NotLogUrls, path) {
			cost := time.Since(start)
			logger.Info(path,
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
				zap.Duration("cost", cost),
			)
		}

	}
}

func GinRecovery(logger *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					logger.Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
					err := c.Error(err.(error))
					if err != nil {
						logger.Error(err.Error())
					}
					c.Abort()
					return
				}

				if stack {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())),
					)
				} else {
					logger.Error("[Recovery from panic]",
						zap.Any("error", err),
						zap.String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

// LogRequest LogUpdate Record the specified HTTP request (including the URL and data) in a log or another location.
func LogRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !lo.Contains([]string{"GET", "HEAD"}, c.Request.Method) {

			data := map[string]any{}
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			_ = c.Request.Body.Close()
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				if valid, err := permissionCheck(c, data); err != nil {
					c.AbortWithStatusJSON(http.StatusBadRequest,
						handler.HttpResponse{Code: http.StatusBadRequest, Message: err.Error()})
					return
				} else if !valid {
					c.AbortWithStatusJSON(http.StatusBadRequest,
						handler.HttpResponse{Code: http.StatusForbidden, Message: "no permission"})
					return
				}
			}

			excludeUrls := map[string]struct{}{}
			if _, ok := excludeUrls[c.Request.RequestURI]; !ok {
				if c.Request.Method != "POST" {
					w := &responseWriter{ResponseWriter: c.Writer, body: []byte{}}
					c.Writer = w
					logger.L.Info("request record", zap.String("body", string(w.body)))
				}
			}
		}
		c.Next()
	}
}

func permissionCheck(ctx *gin.Context, data map[string]any) (bool, error) {
	return true, nil
}
