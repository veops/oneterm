package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/zap"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/api/controller"
	myi18n "github.com/veops/oneterm/i18n"
	"github.com/veops/oneterm/logger"
)

var (
	errUnauthorized = &controller.ApiError{Code: controller.ErrUnauthorized}
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
		var (
			sess   *acl.Session
			err    error
			cookie string
		)

		m := make(map[string]any)
		ctx.ShouldBindBodyWithJSON(&m)
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

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func Error2Resp() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if strings.Contains(ctx.Request.URL.String(), "session/replay") {
			ctx.Next()
			return
		}

		wb := &bodyWriter{
			body:           &bytes.Buffer{},
			ResponseWriter: ctx.Writer,
		}
		ctx.Writer = wb

		ctx.Next()

		obj := make(map[string]any)
		json.Unmarshal(wb.body.Bytes(), &obj)
		if len(ctx.Errors) > 0 {
			if v, ok := obj["code"]; !ok || v == 0 {
				obj["code"] = ctx.Writer.Status()
			}

			if v, ok := obj["message"]; !ok || v == "" {
				e := ctx.Errors.Last().Err
				obj["message"] = e.Error()

				ae, ok := e.(*controller.ApiError)
				if ok {
					lang := ctx.PostForm("lang")
					accept := ctx.GetHeader("Accept-Language")
					localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
					obj["message"] = ae.Message(localizer)

				}
			}
		}
		bs, _ := json.Marshal(obj)
		wb.ResponseWriter.Write(bs)
	}
}
