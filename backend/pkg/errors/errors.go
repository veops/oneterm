package errors

import (
	"encoding/base64"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	myi18n "github.com/veops/oneterm/internal/i18n"
)

const (
	ErrBadRequest             = 4000
	ErrInvalidArgument        = 4001
	ErrDuplicateName          = 4002
	ErrHasChild               = 4003
	ErrHasDepency             = 4004
	ErrNoPerm                 = 4005
	ErrRemoteClient           = 4006
	ErrWrongPk                = 4007
	ErrWrongMac               = 4008
	ErrInvalidSessionId       = 4009
	ErrLogin                  = 4010
	ErrAccessTime             = 4011
	ErrIdleTimeout            = 4012
	ErrWrongPvk               = 4013
	ErrUnauthorized           = 4401
	ErrMFARequired            = 4403
	ErrCredentialInput        = 4404
	ErrPAMIdentity            = 4405
	ErrPAMDenied              = 4406
	ErrPAMConflict            = 4407
	ErrPAMReplay              = 4408
	ErrPAMRevision            = 4409
	ErrPAMInput               = 4411
	ErrPAMBindingChanged      = 4412
	ErrPAMBindingUnavailable  = 4413
	ErrPAMApprovalRequired    = 4414
	ErrPAMRequestState        = 4416
	ErrPAMScopeChanged        = 4418
	ErrPAMProviderUnavailable = 4419
	ErrPAMSessionEnded        = 4420
	ErrPAMSessionCapacity     = 4421
	ErrPAMSessionConflict     = 4422
	ErrTargetAuthentication   = 4432
	ErrTargetHostIdentity     = 4433
	ErrTargetUnavailable      = 4434
	ErrPAMCredentialShared    = 4423
	ErrPAMExecutionState      = 4424
	ErrPAMTemplateDenied      = 4425
	ErrPAMUnavailable         = 5501
	ErrInternal               = 5000
	ErrRemoteServer           = 5001
	ErrConnectServer          = 5002
	ErrLoadSession            = 5003
	ErrAdminClose             = 5004
)

var (
	Err2Msg = map[int]*i18n.Message{
		ErrTargetAuthentication: myi18n.MsgTargetAuthentication,
		ErrTargetHostIdentity:   myi18n.MsgTargetHostIdentity,
		ErrTargetUnavailable:    myi18n.MsgTargetUnavailable,

		ErrBadRequest:             myi18n.MsgBadRequest,
		ErrInvalidArgument:        myi18n.MsgInvalidArguemnt,
		ErrDuplicateName:          myi18n.MsgDupName,
		ErrHasChild:               myi18n.MsgHasChild,
		ErrHasDepency:             myi18n.MsgHasDepdency,
		ErrNoPerm:                 myi18n.MsgNoPerm,
		ErrRemoteClient:           myi18n.MsgRemoteClient,
		ErrWrongPvk:               myi18n.MsgWrongPvk,
		ErrWrongPk:                myi18n.MsgWrongPk,
		ErrWrongMac:               myi18n.MsgWrongMac,
		ErrInvalidSessionId:       myi18n.MsgInvalidSessionId,
		ErrLogin:                  myi18n.MsgLoginError,
		ErrAccessTime:             myi18n.MsgAccessTime,
		ErrIdleTimeout:            myi18n.MsgIdleTimeout,
		ErrUnauthorized:           myi18n.MsgUnauthorized,
		ErrMFARequired:            myi18n.MsgMFARequired,
		ErrCredentialInput:        myi18n.MsgCredentialInput,
		ErrPAMIdentity:            myi18n.MsgPAMIdentity,
		ErrPAMDenied:              myi18n.MsgPAMDenied,
		ErrPAMConflict:            myi18n.MsgPAMConflict,
		ErrPAMReplay:              myi18n.MsgPAMReplay,
		ErrPAMRevision:            myi18n.MsgPAMRevision,
		ErrPAMInput:               myi18n.MsgPAMInput,
		ErrPAMBindingChanged:      myi18n.MsgPAMBindingChanged,
		ErrPAMBindingUnavailable:  myi18n.MsgPAMBindingUnavailable,
		ErrPAMApprovalRequired:    myi18n.MsgPAMApprovalRequired,
		ErrPAMRequestState:        myi18n.MsgPAMRequestState,
		ErrPAMScopeChanged:        myi18n.MsgPAMScopeChanged,
		ErrPAMProviderUnavailable: myi18n.MsgPAMProviderUnavailable,
		ErrPAMSessionEnded:        myi18n.MsgPAMSessionEnded,
		ErrPAMSessionCapacity:     myi18n.MsgPAMSessionCapacity,
		ErrPAMSessionConflict:     myi18n.MsgPAMSessionConflict,
		ErrPAMCredentialShared:    myi18n.MsgPAMCredentialShared,
		ErrPAMExecutionState:      myi18n.MsgPAMExecutionState,
		ErrPAMTemplateDenied:      myi18n.MsgPAMTemplateDenied,
		ErrPAMUnavailable:         myi18n.MsgPAMUnavailable,
		ErrInternal:               myi18n.MsgInternalError,
		ErrRemoteServer:           myi18n.MsgRemoteServer,
		ErrConnectServer:          myi18n.MsgConnectServer,
		ErrLoadSession:            myi18n.MsgLoadSession,
		ErrAdminClose:             myi18n.MsgAdminClose,
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
	lang, accept := "en", "en"
	if ctx != nil {
		lang = ctx.PostForm("lang")
		accept = ctx.GetHeader("Accept-Language")
	}

	localizer := i18n.NewLocalizer(myi18n.Bundle, lang, accept)
	return ae.Message(localizer)
}

func (ae *ApiError) MessageBase64(ctx *gin.Context) string {
	s := ae.MessageWithCtx(ctx)
	return base64.StdEncoding.EncodeToString([]byte(s))
}
