package protocols

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/sshclient"
)

func ConnectSsh(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			select {
			case chs.ErrChan <- err:
			default:
			}
			sess.Stop()
			sess.ClearSSHClient()
			tunneling.CloseTunnels(sess.SessionId)
		}
	}()
	host, port, err := tunneling.Target(sess.Protocol, asset)
	if err != nil {
		return err
	}
	identity := net.JoinHostPort(host, strconv.Itoa(port))
	ip, port, err := tunneling.Proxy(false, sess.SessionId, sess.Protocol, asset, gateway)
	if err != nil {
		return err
	}
	auth, err := repository.GetAuth(account)
	if err != nil {
		return err
	}
	client, err := sshclient.Dial(sess.Gctx, net.JoinHostPort(ip, strconv.Itoa(port)), &gossh.ClientConfig{
		User: account.Account, Auth: []gossh.AuthMethod{auth}, HostKeyCallback: sshclient.HostKey(identity), Timeout: 10 * time.Second})
	if err != nil {
		return err
	}
	sess.SetSSHClient(client)
	sess.SetPAMTransportClose(func() { client.Close() })
	sshSession, err := client.NewSession()
	if err != nil {
		return err
	}
	if sess.Gctx.Err() != nil {
		sshSession.Close()
		return sess.Gctx.Err()
	}
	sshSession.Stdin, sshSession.Stdout, sshSession.Stderr = chs.Rin, chs.Wout, chs.Wout
	w, h := cast.ToInt(ctx.Query("w")), cast.ToInt(ctx.Query("h"))
	if w <= 0 || w > 1000 {
		w = 80
	}
	if h <= 0 || h > 500 {
		h = 24
	}
	if err = sshSession.RequestPty("xterm-256color", h, w, gossh.TerminalModes{gossh.ECHO: 1}); err != nil {
		sshSession.Close()
		return err
	}
	if err = sshSession.Shell(); err != nil {
		sshSession.Close()
		return err
	}
	outputDone := make(chan struct{})
	sess.G.Go(func() error {
		defer close(outputDone)
		for {
			p := make([]byte, 32768)
			n, err := chs.Rout.Read(p)
			if n > 0 && !chs.SendOutput(sess.Gctx, p[:n]) {
				return nil
			}
			if err != nil {
				if errors.Is(err, io.EOF) || sess.Gctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	})
	sess.G.Go(func() error {
		defer sshSession.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			case <-chs.AwayChan:
				return nil
			case window, open := <-chs.WindowChan:
				if !open {
					return nil
				}
				if err := sshSession.WindowChange(window.Height, window.Width); err != nil {
					logger.L().Debug("SSH window change failed", zap.Error(err))
					continue
				}
				if sess.SshRecoder != nil {
					sess.SshRecoder.Resize(window.Width, window.Height)
				}
				if sess.SshParser != nil {
					sess.SshParser.Resize(window.Width, window.Height)
				}
			}
		}
	})
	sess.G.Go(func() error {
		err := sshSession.Wait()
		chs.Wout.Close()
		select {
		case <-outputDone:
		case <-sess.Gctx.Done():
		}
		sess.Once.Do(func() { close(chs.AwayChan) })
		if err != nil && sess.Gctx.Err() == nil {
			return fmt.Errorf("SSH session ended: %w", err)
		}
		return nil
	})
	select {
	case chs.ErrChan <- nil:
	case <-sess.Gctx.Done():
	}
	return nil
}
