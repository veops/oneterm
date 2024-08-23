package sshsrv

import (
	"context"
	"fmt"

	"github.com/gliderlabs/ssh"

	"github.com/veops/oneterm/conf"
)

var (
	ctx, cancel = context.WithCancel(context.Background())
	server      *ssh.Server
)

func init() {
	server = &ssh.Server{
		Addr:            fmt.Sprintf("%s:%d", conf.Cfg.Http.Host, conf.Cfg.Http.Port),
		Handler:         handler,
		BannerHandler:   banner,
		PasswordHandler: func(ctx ssh.Context, password string) bool { return ctx.User() == "root" && password == "root" },
	}
}

func RunSsh() error {
	return server.ListenAndServe()
}

func StopSsh() {
	defer cancel()
}
