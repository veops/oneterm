package sshsrv

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/version"
	"github.com/veops/oneterm/pkg/logger"
)

// testResponseWriter implements http.ResponseWriter for gin.CreateTestContext
type testResponseWriter struct {
	headers http.Header
	body    []byte
	status  int
}

func (w *testResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil
}

func (w *testResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *testResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func handler(sess ssh.Session) {
	defer acl.Logout(sess.Context().Value("session").(*acl.Session))
	pty, _, isPty := sess.Pty()
	if !isPty {
		logger.L().Error("not a pty request")
		return
	}

	// Create a properly initialized gin.Context
	req := &http.Request{
		RemoteAddr: sess.RemoteAddr().String(),
		URL: &url.URL{
			RawQuery: fmt.Sprintf("info=true&w=%d&h=%d", pty.Window.Width, pty.Window.Height),
		},
		Header: make(http.Header),
		Method: "GET",
		Host:   "localhost",
	}

	// Use gin.CreateTestContext to create a properly initialized context
	rec := &testResponseWriter{}
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Request = ctx.Request.WithContext(sess.Context())
	ctx.Set("sessionType", model.SESSIONTYPE_CLIENT)
	ctx.Set("session", sess.Context().Value("session"))

	_, windows, _ := sess.Pty()
	defer sess.Close()
	if err := runTerminal(ctx, newTerminal(pty, sess.RemoteAddr(), sess.Environ()...), sess, sess, windows); err != nil {
		logger.L().Debug("handler stopped", zap.Error(err))
	}
}

func signer() ssh.Signer {
	sysConfigService := service.NewSystemConfigService()

	// Retry logic to wait for database table creation
	var privateKey string
	var err error

	for i := range 10 {
		privateKey, err = sysConfigService.EnsureSSHPrivateKey()
		if err == nil {
			break
		}

		// If table doesn't exist, wait and retry
		if strings.Contains(err.Error(), "doesn't exist") {
			logger.L().Info("Waiting for database initialization...", zap.Int("attempt", i+1))
			time.Sleep(time.Second)
			continue
		}

		// Other errors are fatal
		logger.L().Fatal("failed to ensure SSH private key", zap.Error(err))
	}

	if err != nil {
		logger.L().Fatal("failed to ensure SSH private key after retries", zap.Error(err))
	}

	s, err := gossh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		logger.L().Fatal("failed parse signer", zap.Error(err))
	}
	return s
}

func banner() string {
	// Professional blue theme gradient
	gradient1 := lipgloss.NewStyle().Foreground(lipgloss.Color("#3F75FF")) // Bright primary
	gradient2 := lipgloss.NewStyle().Foreground(lipgloss.Color("#2f54eb")) // Primary color
	gradient3 := lipgloss.NewStyle().Foreground(lipgloss.Color("#7f97fa")) // Light primary
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8c8c8c")).Italic(true)
	bannerText := `
    ██████╗ ███╗   ██╗███████╗████████╗███████╗██████╗ ███╗   ███╗
   ██╔═══██╗████╗  ██║██╔════╝╚══██╔══╝██╔════╝██╔══██╗████╗ ████║
   ██║   ██║██╔██╗ ██║█████╗     ██║   █████╗  ██████╔╝██╔████╔██║
   ██║   ██║██║╚██╗██║██╔══╝     ██║   ██╔══╝  ██╔══██╗██║╚██╔╝██║
   ╚██████╔╝██║ ╚████║███████╗   ██║   ███████╗██║  ██║██║ ╚═╝ ██║
    ╚═════╝ ╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝`

	lines := strings.Split(bannerText, "\n")
	var result strings.Builder

	for i, line := range lines {
		if line == "" {
			result.WriteString("\n")
			continue
		}

		var style lipgloss.Style
		switch {
		case i <= 2:
			style = gradient1
		case i <= 4:
			style = gradient2
		default:
			style = gradient3
		}
		result.WriteString(style.Render(line))
		result.WriteString("\n")
	}

	tagline := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#2f54eb")).
		Bold(true).
		PaddingLeft(15).
		Render("Enterprise Bastion Host Solution")

	versionText := versionStyle.
		PaddingLeft(25).
		Render(version.GetVersion())

	result.WriteString("\n")
	result.WriteString(tagline)
	result.WriteString("  ")
	result.WriteString(versionText)
	result.WriteString("\n")

	return result.String()
}
