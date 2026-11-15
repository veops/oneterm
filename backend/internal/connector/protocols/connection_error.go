package protocols

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh/knownhosts"

	gsession "github.com/veops/oneterm/internal/session"
	apiErrors "github.com/veops/oneterm/pkg/errors"
)

func publicConnectionError(err error, account string) *apiErrors.ApiError {
	var problem *apiErrors.ApiError
	if errors.As(err, &problem) {
		if problem.Code != apiErrors.ErrConnectServer && problem.Code != apiErrors.ErrInternal && problem.Code != apiErrors.ErrRemoteServer {
			data := map[string]any{}
			if permission, ok := problem.Data["perm"].(string); ok {
				for _, action := range []string{"connect", "file_upload", "file_download", "copy", "paste", "share"} {
					if permission == action {
						data["perm"] = action
					}
				}
			}
			return &apiErrors.ApiError{Code: problem.Code, Data: data}
		}
		if cause, ok := problem.Data["err"].(error); ok {
			err = cause
		}
	}
	code := apiErrors.ErrConnectServer
	var hostKey *knownhosts.KeyError
	var network net.Error
	message := err.Error()
	switch {
	case errors.As(err, &hostKey), strings.Contains(message, "knownhosts:"), strings.Contains(message, "host key mismatch"):
		code = apiErrors.ErrTargetHostIdentity
	case strings.Contains(message, "unable to authenticate"):
		code = apiErrors.ErrTargetAuthentication
	case errors.As(err, &network):
		code = apiErrors.ErrTargetUnavailable
	}
	data := map[string]any{}
	if code == apiErrors.ErrTargetAuthentication {
		account = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, account)
		runes := []rune(account)
		if len(runes) > 128 {
			account = string(runes[:128])
		}
		data["account"] = account
	}
	return &apiErrors.ApiError{Code: code, Data: data}
}

// Startup failures must reach the client before Stop closes the socket. Raw
// SSH/database errors may contain remote output and never become UI messages.
func ReportConnectionError(ctx *gin.Context, sess *gsession.Session, ws *websocket.Conn, err error, account string) {
	if ws == nil || err == nil {
		return
	}
	write := func() {
		problem := publicConnectionError(err, account)
		message := problem.MessageWithCtx(ctx)
		var output []byte
		if sess != nil && sess.IsGuacd() {
			output = NewInstruction("error", problem.MessageBase64(ctx), fmt.Sprint(problem.Code)).Bytes()
		} else if ctx.Query("errors") == "true" {
			encoded, _ := json.Marshal(map[string]any{"code": problem.Code, "message": message, "account": problem.Data["account"]})
			output = append([]byte("0error "), encoded...)
		} else {
			output = []byte("\r\n\x1b[31m" + message + "\x1b[0m\r\n")
		}
		// No terminal writer is running until startup succeeds. The once guard
		// serializes the protocol failure and its parent cleanup path.
		ws.SetWriteDeadline(time.Now().Add(time.Second))
		ws.WriteMessage(websocket.TextMessage, output)
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, fmt.Sprintf("oneterm:%d", problem.Code)), time.Now().Add(time.Second))
	}
	if sess == nil {
		write()
		return
	}
	sess.ConnectionErrorOnce.Do(write)
}
