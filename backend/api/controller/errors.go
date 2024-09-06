package controller

import (
	"encoding/base64"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	myi18n "github.com/veops/oneterm/i18n"
)

const (
	ErrBadRequest       = 4000
	ErrInvalidArgument  = 4001
	ErrDuplicateName    = 4002
	ErrHasChild         = 4003
	ErrHasDepency       = 4004
	ErrNoPerm           = 4005
	ErrRemoteClient     = 4006
	ErrWrongPk          = 4007
	ErrWrongMac         = 4008
	ErrInvalidSessionId = 4009
	ErrLogin            = 4010
	ErrAccessTime       = 4011
	ErrIdleTimeout      = 4012
	ErrWrongPvk         = 4013
	ErrUnauthorized     = 4401
	ErrInternal         = 5000
	ErrRemoteServer     = 5001
	ErrConnectServer    = 5002
	ErrLoadSession      = 5003
	ErrAdminClose       = 5004
)

var (
	Err2Msg = map[int]*i18n.Message{
		ErrBadRequest:       myi18n.MsgBadRequest,
		ErrInvalidArgument:  myi18n.MsgInvalidArguemnt,
		ErrDuplicateName:    myi18n.MsgDupName,
		ErrHasChild:         myi18n.MsgHasChild,
		ErrHasDepency:       myi18n.MsgHasDepdency,
		ErrNoPerm:           myi18n.MsgNoPerm,
		ErrRemoteClient:     myi18n.MsgRemoteClient,
		ErrWrongPvk:         myi18n.MsgWrongPvk,
		ErrWrongPk:          myi18n.MsgWrongPk,
		ErrWrongMac:         myi18n.MsgWrongMac,
		ErrInvalidSessionId: myi18n.MsgInvalidSessionId,
		ErrLogin:            myi18n.MsgLoginError,
		ErrAccessTime:       myi18n.MsgAccessTime,
		ErrIdleTimeout:      myi18n.MsgIdleTimeout,
		ErrUnauthorized:     myi18n.MsgUnauthorized,
		ErrInternal:         myi18n.MsgInternalError,
		ErrRemoteServer:     myi18n.MsgRemoteServer,
		ErrConnectServer:    myi18n.MsgConnectServer,
		ErrLoadSession:      myi18n.MsgLoadSession,
		ErrAdminClose:       myi18n.MsgAdminClose,
	}
)

type ApiError struct {
	Code int
	Data map[string]any
}

func (ae *ApiError) Error() string {
	return fmt.Sprintf("code=%d data=%v", ae.Code, ae.Data)
}

func (ae *ApiError) Message(localizer *i18n.Localizer) (msg string) {
	cfg := &i18n.LocalizeConfig{}
	cfg.TemplateData = ae.Data
	m, ok := Err2Msg[ae.Code]
	if !ok {
		msg = ae.Error()
		return
	}
	cfg.DefaultMessage = m

	msg, _ = localizer.Localize(cfg)

	return
}

func (ae *ApiError) MessageWithCtx(ctx *gin.Context) string {
	if ae == nil {
		return ""
	}
	lang := ctx.PostForm("lang")
	accept := ctx.GetHeader("Accept-Language")
	localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
	return ae.Message(localizer)
}

func (ae *ApiError) MessageBase64(ctx *gin.Context) string {
	s := ae.MessageWithCtx(ctx)
	return base64.StdEncoding.EncodeToString([]byte(s))
}
