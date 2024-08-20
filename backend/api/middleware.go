package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
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
		session := &acl.Session{}

		sess, err := ctx.Cookie("session")
		if err == nil && sess != "" {
			s := acl.NewSignature(conf.Cfg.SecretKey, "cookie-session", "", "hmac", nil, nil)
			content, err := s.Unsign(sess)
			if err != nil {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			err = json.Unmarshal(content, &session)
			if err != nil {
				ctx.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			ctx.Set("session", session)
		}

		ctx.Next()
	}
}
