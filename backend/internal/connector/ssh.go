package connector

import (
	"bufio"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	myErrors "github.com/veops/oneterm/pkg/errors"
	"github.com/veops/oneterm/pkg/logger"
)

// connectSsh connects to SSH server
func connectSsh(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
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

	auth, err := service.GetAuth(account)
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

// HandleTerm handles terminal sessions
func HandleTerm(sess *gsession.Session) (err error) {
	defer func() {
		logger.L().Debug("defer HandleTerm", zap.String("sessionId", sess.SessionId))
		sess.SshParser.Close(sess.Prompt)
		sess.Status = model.SESSIONSTATUS_OFFLINE
		sess.ClosedAt = lo.ToPtr(time.Now())
		if err = gsession.UpsertSession(sess); err != nil {
			logger.L().Error("offline session failed", zap.String("sessionId", sess.SessionId), zap.Error(err))
			return
		}
	}()
	chs := sess.Chans
	tk, tk1s, tk1m := time.NewTicker(time.Millisecond*100), time.NewTicker(time.Second), time.NewTicker(time.Minute)
	assetService := service.NewAssetService()
	sess.G.Go(func() error {
		return Read(sess)
	})
	sess.G.Go(func() (err error) {
		defer sess.Chans.Rin.Close()
		defer sess.Chans.Wout.Close()
		for {
			select {
			case <-sess.Gctx.Done():
				Write(sess)
				return
			case <-chs.AwayChan:
				return
			case <-sess.IdleTk.C:
				WriteErrMsg(sess, "idle timeout\n\n")
				return &myErrors.ApiError{Code: myErrors.ErrIdleTimeout, Data: map[string]any{"second": model.GlobalConfig.Load().Timeout}}
			case <-tk1m.C:
				asset, err := assetService.GetById(sess.Gctx, sess.AssetId)
				if err != nil {
					continue
				}
				if CheckTime(asset.AccessAuth) && (sess.ShareId == 0 || time.Now().Before(sess.ShareEnd)) {
					continue
				}
				return &myErrors.ApiError{Code: myErrors.ErrAccessTime}
			case closeBy := <-chs.CloseChan:
				WriteErrMsg(sess, "closed by admin\n\n")
				logger.L().Info("closed by", zap.String("admin", closeBy))
				return &myErrors.ApiError{Code: myErrors.ErrAdminClose, Data: map[string]any{"admin": closeBy}}
			case err = <-chs.ErrChan:
				WriteErrMsg(sess, err.Error())
				return
			case in := <-chs.InChan:
				if sess.SessionType == model.SESSIONTYPE_WEB {
					rt := in[0]
					msg := in[1:]
					switch rt {
					case '1':
						in = msg
					case '9':
						continue
					case 'w':
						wh := strings.Split(string(msg), ",")
						if len(wh) < 2 {
							continue
						}
						chs.WindowChan <- ssh.Window{
							Width:  cast.ToInt(wh[0]),
							Height: cast.ToInt(wh[1]),
						}
						continue
					}
				}
				if cmd, forbidden := sess.SshParser.AddInput(in); forbidden {
					WriteErrMsg(sess, fmt.Sprintf("%s is forbidden\n", cmd))
					sess.SshParser.AddInput(byteClearAll)
					chs.Win.Write(byteClearAll)
					continue
				}
				if _, err = chs.Win.Write(in); err != nil {
					return
				}
			case out := <-chs.OutChan:
				if _, err = chs.OutBuf.Write(out); err != nil {
					return
				}
				sess.SshParser.AddOutput(out)
			case <-tk.C:
				if err = Write(sess); err != nil {
					return
				}
			case <-tk1s.C:
				if sess.Ws == nil {
					continue
				}
				if err = sess.Ws.WriteMessage(websocket.TextMessage, nil); err != nil {
					return
				}
			}
		}
	})

	if err = sess.G.Wait(); err != nil {
		logger.L().Debug("handle term wait end", zap.String("id", sess.SessionId), zap.Error(err))
	}

	return
}
