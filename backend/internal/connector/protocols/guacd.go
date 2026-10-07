package protocols

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/internal/guacd"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

// ConnectGuacd connects to Guacamole server
func ConnectGuacd(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	// Get permissions for guacd connection using batch check
	permissions := &guacd.PermissionInfo{}

	// Check all relevant permissions in one batch call
	if sess.PAMAuthorization != nil {
		initial := sess.GetPermissions()
		permissions.AllowCopy, permissions.AllowPaste = initial.Copy, initial.Paste
		permissions.AllowFileUpload, permissions.AllowFileDownload = initial.FileUpload, initial.FileDownload
	} else {
		batchResult, err := service.DefaultAuthService.HasAuthorizationV2(ctx, sess,
			model.ActionCopy,
			model.ActionPaste,
			model.ActionFileUpload,
			model.ActionFileDownload)

		if err != nil {
			logger.L().Warn("Failed to check permissions, using default settings", zap.Error(err))
			// Continue with default (denied) permissions if check fails
		} else {
			// Extract individual permissions from batch result
			permissions.AllowCopy = batchResult.IsAllowed(model.ActionCopy)
			permissions.AllowPaste = batchResult.IsAllowed(model.ActionPaste)
			permissions.AllowFileUpload = batchResult.IsAllowed(model.ActionFileUpload)
			permissions.AllowFileDownload = batchResult.IsAllowed(model.ActionFileDownload)
		}
	}

	// Clean protocol parameter - remove port number if present for guacd compatibility
	cleanProtocol := sess.Protocol
	if strings.Contains(sess.Protocol, ":") {
		cleanProtocol = strings.Split(sess.Protocol, ":")[0]
	}

	t, err := guacd.NewTunnel("", sess.SessionId, w, h, dpi, cleanProtocol, asset, account, gateway, permissions)
	if err != nil {
		logger.L().Error("guacd tunnel failed", zap.Error(err))
		return
	}
	defer t.Close()
	sess.SetTransportClose(t.Close)

	sess.ConnectionId = t.ConnectionId
	sess.GuacdTunnel = t

	chs.ErrChan <- nil

	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if err != nil {
					if sess.Gctx.Err() != nil {
						return nil
					}
					return err
				}
				if len(p) <= 0 {
					continue
				}
				if !chs.SendOutput(sess.Gctx, p) {
					return nil
				}
			}
		}
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				// Normal termination - return sentinel error
				return ErrSessionClosed
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})

	sess.G.Wait()

	return
}

// HandleGuacd handles Guacamole sessions
func HandleGuacd(sess *gsession.Session, ctx *gin.Context) (err error) {
	defer func() {
		sess.Stop()
		sess.StopIdle()
		gsession.GetOnlineSession().Delete(sess.SessionId)
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if saveErr := gsession.UpsertSession(sess); saveErr != nil {
			logger.L().Error("offline guacd session failed", zap.Error(saveErr))
		}
	}()
	chs := sess.Chans
	sess.G.Go(func() error { return WatchTerminalSession(sess, ctx) })
	sess.G.Go(func() error {
		return Read(sess)
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case err := <-chs.ErrChan:
				return err
			case out := <-chs.OutChan:
				if err := sess.WriteWebsocket(websocket.TextMessage, out); err != nil {
					if sess.Gctx.Err() != nil {
						return nil
					}
					return err
				}
			}
		}
	})

	if err = sess.G.Wait(); err != nil {
		logger.L().Debug("sess wait end guacd", zap.String("id", sess.SessionId), zap.Error(err))
	}

	return
}

// MonitGuacd handles monitoring of Guacamole sessions
func MonitGuacd(ctx *gin.Context, sess *gsession.Session, chs *gsession.SessionChans, ws *websocket.Conn) (err error) {
	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	defer func() {
		chs.ErrChan <- err
	}()

	// For monitoring, no permissions needed since it's read-only
	t, err := guacd.NewTunnel(sess.ConnectionId, "", w, h, dpi, ":", nil, nil, nil, nil)
	if err != nil {
		logger.L().Error("guacd tunnel failed", zap.Error(err))
		return
	}
	defer t.Disconnect()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			default:
				p, err := t.Read()
				if err != nil {
					logger.L().Debug("read instruction failed", zap.Error(err))
					return err
				}
				if len(p) <= 0 {
					continue
				}
				if !chs.SendOutput(gctx, p) {
					return nil
				}
			}
		}
	})
	g.Go(func() error {
		for {
			select {
			case <-sess.Chans.AwayChan:
				err := fmt.Errorf("monitored session closed")
				ws.WriteMessage(websocket.TextMessage, NewInstruction("disconnect", err.Error()).Bytes())
				return err
			case err := <-chs.ErrChan:
				return err
			case out := <-chs.OutChan:
				ws.WriteMessage(websocket.TextMessage, out)
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})
	if err = g.Wait(); err != nil {
		logger.L().Warn("monit guacd stopped", zap.Error(err))
	}

	return
}
