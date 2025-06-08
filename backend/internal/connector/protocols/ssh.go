package protocols

import (
	"bufio"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

// ConnectSsh connects to SSH server
func ConnectSsh(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	ip, port, err := tunneling.Proxy(false, sess.SessionId, "ssh", asset, gateway)
	if err != nil {
		return
	}

	auth, err := repository.GetAuth(account)
	if err != nil {
		return
	}

	sshCli, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &gossh.ClientConfig{
		User:            account.Account,
		Auth:            []gossh.AuthMethod{auth},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		logger.L().Error("ssh dial failed", zap.Error(err))
		return
	}

	// CRITICAL: Store SSH client in session for file transfer reuse
	sess.SetSSHClient(sshCli)
	logger.L().Info("SSH client stored in session for reuse", zap.String("sessionId", sess.SessionId))

	sshSess, err := sshCli.NewSession()
	if err != nil {
		logger.L().Error("ssh session create failed", zap.Error(err))
		return
	}
	defer sshSess.Close()

	sshSess.Stdin = chs.Rin
	sshSess.Stdout = chs.Wout
	sshSess.Stderr = chs.Wout

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err = sshSess.RequestPty("xterm", h, w, modes); err != nil {
		logger.L().Error("ssh request pty failed", zap.Error(err))
		return
	}
	if err = sshSess.Shell(); err != nil {
		logger.L().Error("ssh start shell failed", zap.Error(err))
		return
	}

	sess.G.Go(func() error {
		err = sshSess.Wait()
		return fmt.Errorf("ssh session wait end %w", err)
	})

	chs.ErrChan <- err

	sess.G.Go(func() error {
		buf := bufio.NewReader(chs.Rout)
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				rn, size, err := buf.ReadRune()
				if err != nil {
					return err
				}
				if size <= 0 || rn == utf8.RuneError {
					continue
				}
				p := make([]byte, utf8.RuneLen(rn))
				utf8.EncodeRune(p, rn)
				chs.OutChan <- p
			}
		}
	})
	sess.G.Go(func() error {
		defer sshSess.Close()
		defer sess.Chans.Rout.Close()
		defer sess.Chans.Win.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return fmt.Errorf("away")
			case window := <-chs.WindowChan:
				if err := sshSess.WindowChange(window.Height, window.Width); err != nil {
					logger.L().Warn("reset window size failed", zap.Error(err))
					continue
				}
				sess.SshRecoder.Resize(window.Width, window.Height)
				sess.SshParser.Resize(window.Width, window.Height)
			}
		}
	})

	sess.G.Wait()

	return
}
