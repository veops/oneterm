package sshsrv

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/getwe/figlet4go"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
)

func handler(sess ssh.Session) {
	defer acl.Logout(sess.Context().Value("session").(*acl.Session))
	pty, _, isPty := sess.Pty()
	if !isPty {
		logger.L().Error("not a pty request")
		return
	}

	ctx := &gin.Context{
		Request: &http.Request{
			RemoteAddr: sess.RemoteAddr().String(),
			URL: &url.URL{
				RawQuery: fmt.Sprintf("info=true&w=%d&h=%d", pty.Window.Width, pty.Window.Height),
			},
		},
	}
	ctx.Set("sessionType", model.SESSIONTYPE_CLIENT)
	ctx.Set("session", sess.Context().Value("session"))

	eg, gctx := errgroup.WithContext(sess.Context())
	r, w := io.Pipe()
	eg.Go(func() error {
		_, err := io.Copy(w, sess)
		return err
	})
	eg.Go(func() error {
		defer sess.Close()
		defer r.Close()
		defer w.Close()
		vw := initialView(ctx, sess, r, w, gctx)
		defer vw.RecordHisCmd()
		p := tea.NewProgram(vw, tea.WithContext(gctx), tea.WithInput(r), tea.WithOutput(sess))
		_, err := p.Run()

		return err
	})

	if err := eg.Wait(); err != nil {
		logger.L().Debug("handler stopped", zap.Error(err))
	}
}

func signer() ssh.Signer {
	s, err := gossh.ParsePrivateKey([]byte(config.Cfg.Ssh.PrivateKey))
	if err != nil {
		logger.L().Fatal("failed parse signer", zap.Error(err))
	}
	return s
}

func banner() string {
	str := "ONETERM"
	ascii := figlet4go.NewAsciiRender()
	colors := [...]color.Attribute{
		color.FgMagenta,
		color.FgYellow,
		color.FgBlue,
		color.FgCyan,
		color.FgRed,
		color.FgWhite,
		color.FgGreen,
	}
	options := figlet4go.NewRenderOptions()
	options.FontColor = make([]color.Attribute, len(str))
	for i := range options.FontColor {
		options.FontColor[i] = colors[i%len(colors)]
	}
	renderStr, _ := ascii.RenderOpts(str, options)
	return renderStr
}
