package protocols

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/guacd"
	myi18n "github.com/veops/oneterm/internal/i18n"
	"github.com/veops/oneterm/internal/model"
	fileservice "github.com/veops/oneterm/internal/service/file"
	gsession "github.com/veops/oneterm/internal/session"
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

var (
	// ErrSessionClosed is a sentinel error for normal session termination
	// This is returned when a session is closed normally (e.g., user exits)
	ErrSessionClosed = errors.New("session closed normally")

	Upgrader = websocket.Upgrader{
		HandshakeTimeout: time.Minute,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
)

func TerminalMessage(data []byte) ([]byte, ssh.Window) {
	if len(data) == 0 {
		return nil, ssh.Window{}
	}
	switch data[0] {
	case '1':
		return data[1:], ssh.Window{}
	case 'w':
		parts := strings.Split(string(data[1:]), ",")
		if len(parts) != 2 {
			break
		}
		w, werr := strconv.Atoi(parts[0])
		h, herr := strconv.Atoi(parts[1])
		if werr == nil && herr == nil && w > 0 && h > 0 && w <= 1000 && h <= 500 {
			return nil, ssh.Window{Width: w, Height: h}
		}
	}
	return nil, ssh.Window{}
}

// WriteToMonitors sends data to all monitoring sessions
func WriteToMonitors(monitors *sync.Map, out []byte) {
	if len(out) == 0 {
		return
	}
	var snapshot []byte
	monitors.Range(func(k, v any) bool {
		if monitor, ok := v.(*Monitor); ok {
			if snapshot == nil {
				snapshot = append([]byte(nil), out...)
			}
			monitor.Write(snapshot)
			return true
		}
		ws, ok := v.(*websocket.Conn)
		if ok && ws != nil {
			ws.WriteMessage(websocket.TextMessage, out)
		}
		return true
	})
}

// CheckTime checks if the current time is within the allowed time range
func CheckTime(data model.AccessAuth) bool {
	now := time.Now()
	in := true
	if (data.Start != nil && now.Before(*data.Start)) || (data.End != nil && now.After(*data.End)) {
		in = false
	}
	if !in {
		return false
	}
	in = false
	has := false
	week, hm := now.Weekday(), now.Format("15:04")
	for _, r := range data.Ranges {
		has = has || len(r.Times) > 0
		if (r.Week+1)%7 == int(week) {
			for _, str := range r.Times {
				ss := strings.Split(str, "~")
				in = in || (len(ss) >= 2 && hm >= ss[0] && hm <= ss[1])
			}
		}
	}
	return !has || in == data.Allow
}

// HandleError handles errors from sessions
func HandleError(ctx *gin.Context, sess *gsession.Session, err error, ws *websocket.Conn, chs *gsession.SessionChans) {
	defer func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Error("Recovered from panic in HandleError",
					zap.Any("panic", r),
					zap.String("sessionId", func() string {
						if sess != nil {
							return sess.SessionId
						}
						return ""
					}()))
			}
		}()

		if sess == nil || sess.Chans == nil {
			return
		}
		if chs != nil {
			chs.CloseAway()
			return
		}

		sess.Once.Do(func() {
			logger.L().Debug("Closing AwayChan from HandleError",
				zap.String("sessionId", sess.SessionId))
			close(sess.Chans.AwayChan)
		})
	}()

	if chs != nil {
		return
	}
	if err == nil {
		return
	}
	if sess != nil {
		logger.L().Debug("", zap.String("session_id", sess.SessionId), zap.Error(err))
	}
	ae, ok := err.(*myErrors.ApiError)
	if sess != nil && sess.IsGuacd() && ws != nil {
		ws.WriteMessage(websocket.TextMessage, NewInstruction("error", lo.Ternary(ok, (ae).MessageBase64(ctx), err.Error()), cast.ToString(myErrors.ErrAdminClose)).Bytes())
	} else if sess != nil {
		if ctx.Query("is_monitor") == "true" {
			return
		}
		WriteErrMsg(sess, lo.Ternary(ok, ae.MessageWithCtx(ctx), err.Error()))
	}
}

// WriteErrMsg writes an error message to the session
func WriteErrMsg(sess *gsession.Session, msg string, queued ...bool) {
	chs := sess.Chans
	out := []byte(fmt.Sprintf("\r\n \033[31m %s \x1b[0m", msg))
	if len(queued) > 0 && queued[0] && chs.OutBuf.Len()+len(out) > 1024*1024 {
		sess.Stop()
		return
	}
	chs.OutBuf.Write(out)
	if len(queued) == 0 || !queued[0] {
		Write(sess)
	}
}

// Write writes data to the session output
// skipRecording: If true, it will skip recording to avoid duplicate recordings of manually recorded content
func Write(sess *gsession.Session, skipRecording ...bool) (err error) {
	chs := sess.Chans
	out := chs.OutBuf.Drain()

	if sess.SessionType == model.SESSIONTYPE_WEB && sess.Ws != nil {
		if len(out) > 0 || sess.IsGuacd() {
			err = sess.WriteTerminal(out)
		}
	} else if sess.SessionType == model.SESSIONTYPE_CLIENT && len(out) > 0 {
		_, err = sess.CliRw.WriteContext(sess.Gctx, out)
	}

	// Only write to recording if skipRecording is not specified or explicitly set to false
	shouldSkip := len(skipRecording) > 0 && skipRecording[0]
	if sess.SshRecoder != nil && len(out) > 0 && !sess.IsGuacd() && !shouldSkip {
		sess.SshRecoder.Write(out)
	}

	WriteToMonitors(sess.Monitors, out)

	return
}

// Read reads data from the session input
func Read(sess *gsession.Session) error {
	chs := sess.Chans

	// Handle CLIENT type with non-blocking reads
	if sess.SessionType == model.SESSIONTYPE_CLIENT {
		readChan := make(chan []byte)
		errChan := make(chan error)

		go func() {
			for {
				p, err := sess.CliRw.Read()
				if err != nil {
					select {
					case errChan <- err:
					case <-sess.Gctx.Done():
					case <-chs.AwayChan:
					}
					return
				}
				select {
				case readChan <- p:
				case <-sess.Gctx.Done():
					return
				case <-chs.AwayChan:
					return
				}
			}
		}()

		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-sess.Chans.AwayChan:
				return nil
			case err := <-errChan:
				if sess.Gctx.Err() != nil {
					return nil
				}
				return err
			case p := <-readChan:
				select {
				case chs.InChan <- p:
					sess.SetIdle()
				case <-sess.Gctx.Done():
					return nil
				case <-chs.AwayChan:
					return nil
				}
			}
		}
	}

	// Original logic for WEB type
	away := chs.AwayChan
	if sess.OutputFlow != nil {
		// Read acknowledgements until final output has drained.
		away = nil
	}
	for {
		select {
		case <-sess.Gctx.Done():
			return nil
		case <-away:
			return nil
		default:
			if sess.SessionType == model.SESSIONTYPE_WEB {
				sess.Ws.SetReadLimit(1024 * 1024)
				sess.Ws.SetReadDeadline(time.Now().Add(max(2*time.Minute, gsession.IdleTimeout())))
				t, msg, err := sess.Ws.ReadMessage()
				if err != nil {
					if sess.Gctx.Err() != nil {
						return nil
					}
					return err
				}
				if len(msg) <= 0 {
					continue
				}
				switch t {
				case websocket.TextMessage:
					if handled, err := sess.HandleOutputAck(msg); handled {
						if err != nil {
							return err
						}
						continue
					}
					select {
					case <-chs.AwayChan:
						continue
					default:
					}
					select {
					case chs.InChan <- msg:
					case <-sess.Gctx.Done():
						return nil
					case <-chs.AwayChan:
						if sess.OutputFlow != nil {
							continue
						}
						return nil
					}
					input, window := TerminalMessage(msg)
					active := len(input) > 0 || window.Width > 0
					if sess.IsGuacd() {
						active = guacd.IsActive(msg)
					}
					if msg[0] != '9' && active {
						sess.SetIdle()
					}
				}
			}
		}
	}
}

// Instruction represents a Guacamole instruction
type Instruction struct {
	opcode string
	args   []string
}

// NewInstruction creates a new Guacamole instruction
func NewInstruction(opcode string, args ...string) *Instruction {
	return &Instruction{
		opcode: opcode,
		args:   args,
	}
}

// Bytes converts the instruction to a byte array
func (instr *Instruction) Bytes() []byte {
	result := instr.opcode
	for _, arg := range instr.args {
		result += "," + fmt.Sprintf("%d", len(arg)) + "." + arg
	}
	result += ";"
	return []byte(result)
}

// IsActive checks if a message indicates user activity
func IsActive(message []byte) bool {
	return len(message) > 0
}

// OfflineSession makes a session offline
func OfflineSession(ctx *gin.Context, sessionId string, closer string) {
	logger.L().Debug("offline", zap.String("session_id", sessionId), zap.String("closer", closer))
	defer gsession.GetOnlineSession().Delete(sessionId)

	// Clean up session-based file client
	fileservice.DefaultFileService.CloseSessionFileClient(sessionId)

	session := gsession.GetOnlineSessionById(sessionId)
	if session == nil {
		return
	}
	if closer != "" && session.Chans != nil {
		select {
		case session.Chans.CloseChan <- closer:
			break
		case <-time.After(time.Second):
			break
		}
	}
	session.Monitors.Range(func(key, value any) bool {
		if monitor, ok := value.(*Monitor); ok {
			monitor.Close()
			return true
		}
		ws, ok := value.(*websocket.Conn)
		if ok && ws != nil {
			lang := ctx.PostForm("lang")
			accept := ctx.GetHeader("Accept-Language")
			localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
			cfg := &i18n.LocalizeConfig{
				TemplateData:   map[string]any{"sessionId": sessionId},
				DefaultMessage: myi18n.MsgSessionEnd,
			}
			msg, _ := localizer.Localize(cfg)
			ws.WriteMessage(websocket.TextMessage, []byte(msg))
			ws.Close()
		}
		return true
	})
}
