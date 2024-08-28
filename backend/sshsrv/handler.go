package sshsrv

import (
	"fmt"
	"net/http"
	"net/url"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/getwe/figlet4go"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

func handler(sess ssh.Session) {
	pty, _, isPty := sess.Pty()
	if !isPty {
		logger.L().Error("not a pty request")
		return
	}

	ctx := &gin.Context{
		Request: &http.Request{
			RemoteAddr: sess.RemoteAddr().String(),
			URL: &url.URL{
				RawQuery: fmt.Sprintf("w=%d&h=%d", pty.Window.Width, pty.Window.Height),
			},
		},
	}
	ctx.Set("sessionType", model.SESSIONTYPE_CLIENT)
	ctx.Set("session", sess.Context().Value("session"))

	p := tea.NewProgram(initialView(ctx, sess), tea.WithInput(sess), tea.WithOutput(sess))

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
}

func signer() ssh.Signer {
	str := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBg490b4zqumtizCyM4RWtzJnPEsPIInBFugk8+UCb8XgAAAKCc1yKrnNci
qwAAAAtzc2gtZWQyNTUxOQAAACBg490b4zqumtizCyM4RWtzJnPEsPIInBFugk8+UCb8Xg
AAAECvd1Yj+bQxyxJtU3PirLK68CD3MWqBv0/shlFKS6wmbWDj3RvjOq6a2LMLIzhFa3Mm
c8Sw8gicEW6CTz5QJvxeAAAAGnJvb3RAbG9jYWxob3N0LmxvY2FsZG9tYWluAQID
-----END OPENSSH PRIVATE KEY-----
	`
	s, err := gossh.ParsePrivateKey([]byte(str))
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
