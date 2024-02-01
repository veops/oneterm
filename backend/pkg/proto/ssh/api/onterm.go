package api

import (
	gossh "github.com/gliderlabs/ssh"
)

type CoreInstance struct {
	Auth    *Auth
	Asset   *AssetCore
	Session *gossh.Session
	Audit   *AuditCore
}

func NewCoreInstance(apiHost, token, secretKey string) *CoreInstance {
	coreInstance := &CoreInstance{
		Auth:  NewAuthServer("", "", "", apiHost, token, secretKey),
		Asset: NewAssetServer(apiHost, token),
	}
	return coreInstance
}
