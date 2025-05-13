package connector

import (
	"fmt"
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
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

// connectGuacd connects to Guacamole server
func connectGuacd(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	w, h, dpi := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h")), cast.ToInt(ctx.Query("dpi"))

	t, err := guacd.NewTunnel("", sess.SessionId, w, h, dpi, sess.Protocol, asset, account, gateway)
	if err != nil {
		logger.L().Error("guacd tunnel failed", zap.Error(err))
		return
	}
	defer t.Close()

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
					return err
				}
				if len(p) <= 0 {
					continue
				}
				chs.OutChan <- p
			}
		}
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case in := <-chs.InChan:
				t.Write(in)
			}
		}
	})

	sess.G.Wait()

	return
}

// HandleGuacd handles Guacamole sessions
func HandleGuacd(sess *gsession.Session) (err error) {
	defer func() {
		sess.GuacdTunnel.Disconnect()
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if err = gsession.UpsertSession(sess); err != nil {
			logger.L().Error("offline guacd session failed", zap.Error(err))
			return
		}
	}()
	chs := sess.Chans
	tk := time.NewTicker(time.Minute)
	assetService := service.NewAssetService()
	sess.G.Go(func() error {
		return Read(sess)
	})
	sess.G.Go(func() error {
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-sess.IdleTk.C:
				return &myErrors.ApiError{Code: myErrors.ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}
			case <-tk.C:
				asset, err := assetService.GetById(sess.Gctx, sess.AssetId)
				if err != nil {
					continue
				}
				if CheckTime(asset.AccessAuth) && (sess.ShareId == 0 || time.Now().Before(sess.ShareEnd)) {
					continue
				}
				return &myErrors.ApiError{Code: myErrors.ErrAccessTime}
			case closeBy := <-chs.CloseChan:
				return &myErrors.ApiError{Code: myErrors.ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err := <-chs.ErrChan:
				return err
			case out := <-chs.OutChan:
				sess.Ws.WriteMessage(websocket.TextMessage, out)
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

	t, err := guacd.NewTunnel(sess.ConnectionId, "", w, h, dpi, ":", nil, nil, nil)
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
				chs.OutChan <- p
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
