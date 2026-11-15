package service

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
)

var pamAuthorizationEngine = &gin.Engine{ContextWithFallback: true}

func checkPAMStandingConnection(ctx context.Context, permit *model.PAMConnectionPermit) error {
	actor, err := acl.PAMCurrentUser(ctx, permit.UID)
	if err != nil {
		return ErrPAMDenied
	}
	if DefaultAuthService == nil {
		return ErrPAMUnavailable
	}
	request := (&http.Request{RemoteAddr: net.JoinHostPort(permit.SourceIP, "0"), Header: make(http.Header)}).WithContext(ctx)
	authorization := gin.CreateTestContextOnly(nil, pamAuthorizationEngine)
	authorization.Request = request
	authorization.Set("session", actor)
	result, err := DefaultAuthService.HasStandingAuthorizationV2(authorization, &gsession.Session{Session: &model.Session{AssetId: permit.AssetID, AccountId: permit.AccountID}}, model.ActionConnect)
	if err != nil {
		return ErrPAMUnavailable
	}
	if result == nil || !result.IsAllowed(model.ActionConnect) {
		return ErrPAMDenied
	}
	return nil
}
