package sshsrv

import (
	"context"
	"fmt"

	"github.com/gliderlabs/ssh"

	"github.com/veops/oneterm/acl"
	"github.com/veops/oneterm/conf"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	server      *ssh.Server
)

func init() {
	server = &ssh.Server{
		Addr:    fmt.Sprintf("%s:%d", conf.Cfg.Ssh.Host, conf.Cfg.Ssh.Port),
		Handler: handler,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			sess, err := acl.LoginByPassword(ctx, ctx.User(), password)
			ctx.SetValue("session", sess)
			return err == nil
		},
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			sess, err := acl.LoginByPublicKey(ctx, ctx.User())
			ctx.SetValue("session", sess)
			return err == nil
		},
		HostSigners: []ssh.Signer{signer()},
	}
}

func RunSsh() error {
	return server.ListenAndServe()
}

func StopSsh() {
	defer cancel()
}
