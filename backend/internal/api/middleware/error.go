package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	myi18n "github.com/veops/oneterm/internal/i18n"
	"github.com/veops/oneterm/pkg/errors"
)

type bodyWriter struct {
	gin.ResponseWriter
	body     *bytes.Buffer
	hijacked bool
}

func (w bodyWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *bodyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	connection, buffer, err := w.ResponseWriter.Hijack()
	if err == nil {
		w.hijacked = true
	}
	return connection, buffer, err
}

func Error2RespMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Skip middleware for session replay and file download endpoints
		urlPath := ctx.Request.URL.String()
		if strings.Contains(urlPath, "session/replay") ||
			strings.Contains(urlPath, "/download") {
			ctx.Next()
			return
		}

		wb := &bodyWriter{
			body:           &bytes.Buffer{},
			ResponseWriter: ctx.Writer,
		}
		ctx.Writer = wb

		ctx.Next()
		if wb.hijacked {
			return
		}

		obj := make(map[string]any)
		json.Unmarshal(wb.body.Bytes(), &obj)
		if len(ctx.Errors) > 0 {
			e := ctx.Errors.Last().Err
			ae, apiError := e.(*errors.ApiError)
			if v, ok := obj["code"]; !ok || v == 0 {
				if apiError {
					obj["code"] = ae.Code
				} else {
					obj["code"] = ctx.Writer.Status()
				}
			}

			if v, ok := obj["message"]; !ok || v == "" {
				obj["message"] = e.Error()

				if apiError {
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
