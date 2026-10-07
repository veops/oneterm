package protocols

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	myErrors "github.com/veops/oneterm/pkg/errors"
)

func WatchTerminalSession(sess *gsession.Session, ctx *gin.Context) error {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	away := sess.Chans.AwayChan
	for {
		var err error
		select {
		case <-sess.Gctx.Done():
			return nil
		case <-away:
			away = nil
			continue
		case <-sess.IdleTk.C:
			seconds := 3600
			if cfg := model.GlobalConfig.Load(); cfg != nil && cfg.Timeout > 0 {
				seconds = cfg.Timeout
			}
			err = &myErrors.ApiError{Code: myErrors.ErrIdleTimeout, Data: map[string]any{"second": seconds}}
		case closer := <-sess.Chans.CloseChan:
			err = &myErrors.ApiError{Code: myErrors.ErrAdminClose, Data: map[string]any{"admin": closer}}
		case <-tick.C:
			if sess.AssetId == 0 {
				continue
			}
			err = checkTerminalAccess(sess, ctx)
			if err == nil {
				continue
			}
		}
		sess.Stop()
		return err
	}
}

func checkTerminalAccess(sess *gsession.Session, ctx *gin.Context) error {
	asset, err := service.NewAssetService().GetById(sess.Gctx, sess.AssetId)
	if err != nil {
		return err
	}
	if !CheckTime(asset.AccessAuth) || sess.ShareId != 0 && time.Now().After(sess.ShareEnd) {
		return &myErrors.ApiError{Code: myErrors.ErrAccessTime}
	}
	if sess.PAMAuthorization != nil {
		return nil
	}
	if ctx == nil {
		return &myErrors.ApiError{Code: myErrors.ErrUnauthorized}
	}
	snapshot := &gsession.Session{Session: &model.Session{Asset: asset, AssetId: sess.AssetId, AccountId: sess.AccountId, ShareId: sess.ShareId}}
	result, err := service.DefaultAuthService.HasStandingAuthorizationV2(ctx, snapshot, model.ActionConnect)
	if err != nil {
		return err
	}
	if !result.IsAllowed(model.ActionConnect) {
		return &myErrors.ApiError{Code: myErrors.ErrUnauthorized, Data: map[string]any{"perm": "connect"}}
	}
	return nil
}
