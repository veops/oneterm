package connect

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

// connectOther connects to other protocols (Redis, MySQL, etc.)
func connectOther(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	// Handle Redis connections
	if sess.IsRedis() {
		logger.L().Info("Starting Redis connection", zap.String("sessionId", sess.SessionId))

		// Setup proxy and connection parameters
		protocol := strings.Split(sess.Protocol, ":")[0]
		ip, port, err := tunneling.Proxy(false, sess.SessionId, protocol, asset, gateway)
		if err != nil {
			logger.L().Error("Failed to setup tunnel", zap.Error(err))
			return err
		}

		// Build redis-cli command
		args := []string{"-h", ip, "-p", fmt.Sprintf("%d", port)}
		if account.Password != "" {
			args = append(args, "-a", account.Password)
		}
		logger.L().Info("Starting redis-cli", zap.String("host", ip), zap.Int("port", port))

		// Create command and pseudo-terminal
		cmd := exec.CommandContext(sess.Gctx, "redis-cli", args...)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		ptmx, err := pty.Start(cmd)
		if err != nil {
			logger.L().Error("Failed to start redis-cli with pty", zap.Error(err))
			return fmt.Errorf("failed to start redis-cli: %w", err)
		}

		// Set standard terminal size
		_ = pty.Setsize(ptmx, &pty.Winsize{
			Cols: 80,
			Rows: 24,
		})

		// Simplified IO channel setup - direct connection
		chs.Rin, chs.Win = io.Pipe()

		// Create a reader to read PTY output
		ptmxReader := bufio.NewReader(ptmx)

		// Add an atomic variable to track if exit message has been sent
		var exitMessageSent int32

		// Monitor process exit
		sess.G.Go(func() error {
			err := cmd.Wait()
			logger.L().Info("Redis cli process exited", zap.Error(err))

			// Only send termination message if not already sent
			if atomic.CompareAndSwapInt32(&exitMessageSent, 0, 1) {
				// Send termination message
				terminationMsg := "\r\n\033[31mThe connection is closed!\033[0m\r\n"
				chs.OutBuf.WriteString(terminationMsg)
			}

			sess.Once.Do(func() {
				logger.L().Info("Closing AwayChan from Redis process monitor")
				close(chs.AwayChan)
			})
			return fmt.Errorf("redis-cli process terminated: %w", err)
		})

		// Goroutine 1: Process input, detect exit command
		sess.G.Go(func() error {
			defer ptmx.Close()
			buf := make([]byte, 1024)
			var inputBuffer string

			for {
				select {
				case <-sess.Gctx.Done():
					return nil
				default:
					n, err := chs.Rin.Read(buf)
					if err != nil {
						if err == io.EOF {
							return nil
						}
						return err
					}
					if n > 0 {
						input := string(buf[:n])

						// Accumulate user input to detect complete commands
						if input != "\r" {
							// Check if all characters are printable
							allPrintable := true
							for _, ch := range input {
								if ch < 32 || ch > 126 {
									allPrintable = false
									break
								}
							}
							if allPrintable {
								inputBuffer += input
							}
						}

						// Detect command end (enter key)
						if input == "\r" {
							processCmd := strings.TrimSpace(inputBuffer)

							// Check for exit command
							if strings.EqualFold(processCmd, "exit") || strings.EqualFold(processCmd, "quit") {
								// Send command to Redis CLI for normal exit
								if _, err := ptmx.Write(buf[:n]); err != nil {
									return err
								}

								// Mark exit message as sent, but don't send it here. Let the process exit handler send it.
								atomic.StoreInt32(&exitMessageSent, 1)

								inputBuffer = ""
								continue
							}

							// Reset command buffer
							inputBuffer = ""
						}

						// Forward input to Redis CLI
						if _, err := ptmx.Write(buf[:n]); err != nil {
							return err
						}
					}
				}
			}
		})

		// Goroutine 2: Read Redis CLI output and send to OutChan
		sess.G.Go(func() error {
			for {
				select {
				case <-sess.Gctx.Done():
					return nil
				default:
					rn, size, err := ptmxReader.ReadRune()
					if err != nil {
						if err == io.EOF {
							return nil
						}
						return err
					}
					if size <= 0 || rn == utf8.RuneError {
						continue
					}

					p := make([]byte, utf8.RuneLen(rn))
					utf8.EncodeRune(p, rn)

					// Send to OutChan for HandleTerm processing
					chs.OutChan <- p
				}
			}
		})

		// Goroutine 3: Handle window size changes
		sess.G.Go(func() error {
			for {
				select {
				case <-sess.Gctx.Done():
					return nil
				case <-chs.AwayChan:
					return fmt.Errorf("away")
				case window := <-chs.WindowChan:
					// Adjust Redis terminal size
					_ = pty.Setsize(ptmx, &pty.Winsize{
						Cols: uint16(window.Width),
						Rows: uint16(window.Height),
					})

					// Adjust parser size
					if sess.SshParser != nil {
						sess.SshParser.Resize(window.Width, window.Height)
					}
				}
			}
		})

		// Notify connection is ready
		chs.ErrChan <- nil
		return nil
	}

	return fmt.Errorf("unsupported protocol: %s", sess.Protocol)
}
