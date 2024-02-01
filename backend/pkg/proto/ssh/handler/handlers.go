package handler

import (
	"io"
	"os"
	"strings"
	"time"

	gossh "github.com/gliderlabs/ssh"
	gssh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/api"
	cfg "github.com/veops/oneterm/pkg/proto/ssh/config"
	"github.com/veops/oneterm/pkg/server/model"
)

type sshdServer struct {
	Core *api.CoreInstance
}

func Init(address, apiHost, token, privateKeyPath, secretKey string) (*gossh.Server, error) {
	sshd := NewSshdServer(apiHost, token, secretKey)
	s := &gossh.Server{
		Addr:             address,
		Handler:          sshd.HomeHandler,
		PasswordHandler:  sshd.PasswordHandler,
		PublicKeyHandler: sshd.PublicKeyHandler,
		IdleTimeout:      time.Hour*2 + time.Minute,
	}

	for _, v := range hostPrivateKeys(privateKeyPath) {
		singer, er := gssh.ParsePrivateKey(v)
		if er != nil {
			continue
		}
		s.AddHostKey(singer)
	}
	return s, nil
}

func hostPrivateKeys(privateKeyPath string) [][]byte {
	var res [][]byte

	if privateKeyPath == "" {
		homeDir, er := os.UserHomeDir()
		if er != nil {
			logger.L.Error(er.Error())
		}
		privateKeyPath = homeDir + "/.ssh/id_ed25519"
	}
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		logger.L.Error(err.Error())
		return res
	}
	return [][]byte{privateKey}
}

func NewSshdServer(apiHost, token, secretKey string) *sshdServer {
	s := &sshdServer{
		Core: api.NewCoreInstance(apiHost, token, secretKey),
	}
	return s
}

func (s *sshdServer) PasswordHandler(ctx gossh.Context, password string) bool {
	if password == "" {
		return false
	}

	if ctx.User() == cfg.SSHConfig.WebUser && password == cfg.SSHConfig.WebToken {
		ctx.SetValue("sshType", model.SESSIONTYPE_WEB)
		return true
	}
	ctx.SetValue("sshType", model.SESSIONTYPE_CLIENT)
	s.Core.Auth.Username = ctx.User()
	s.Core.Auth.Password = password
	s.Core.Auth.PublicKey = ""
	return s.Auth(ctx)
}

func (s *sshdServer) PublicKeyHandler(ctx gossh.Context, key gossh.PublicKey) bool {
	authorizedKey := gssh.MarshalAuthorizedKey(key)
	s.Core.Auth.PublicKey = strings.TrimSpace(string(authorizedKey))
	if s.Core.Auth.PublicKey == "" {
		return false
	}
	s.Core.Auth.Username = ctx.User()
	s.Core.Auth.Password = ""
	if ctx.Value("sshType") == nil {
		ctx.SetValue("sshType", model.SESSIONTYPE_CLIENT)
	}
	return s.Auth(ctx)
}

func (s *sshdServer) Auth(ctx gossh.Context) bool {
	cookie, err := s.Core.Auth.Authenticate()
	if err != nil || cookie == "" {
		return false
	}

	ctx.SetValue("cookie", cookie)
	return true
}

func (s *sshdServer) HomeHandler(gs gossh.Session) {

	if py, winChan, isPty := gs.Pty(); isPty {
		if py.Window.Height == 0 {
			py.Window.Height = 24
		}
		interactiveSrv := NewInteractiveHandler(gs, s, py)
		go interactiveSrv.WatchWinSize(winChan)
		interactiveSrv.Schedule(&py)
	} else {
		if _, err := io.WriteString(gs, "不是PTY请求.\n"); err != nil {
			logger.L.Error(err.Error())
		}
		err := gs.Exit(1)
		if err != nil {
			logger.L.Error(err.Error())
		}
		return
	}
}
