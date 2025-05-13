package db

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
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

// connectDB connects to other protocols (Redis, MySQL, PostgreSQL, MongoDB etc.)
func connectDB(sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	// Extract protocol from session
	protocol := strings.Split(sess.Protocol, ":")[0]
	logger.L().Info("Starting database connection", zap.String("protocol", protocol), zap.String("sessionId", sess.SessionId))

	// Setup proxy and connection parameters
	ip, port, err := tunneling.Proxy(false, sess.SessionId, protocol, asset, gateway)
	if err != nil {
		logger.L().Error("Failed to setup tunnel", zap.Error(err))
		return err
	}

	// Configure client based on protocol
	var clientConfig DBClientConfig

	switch {
	case sess.IsRedis():
		clientConfig = getRedisConfig(ip, port, account)
	case sess.IsMysql():
		clientConfig = getMySQLConfig(ip, port, account)
	case strings.HasPrefix(sess.Protocol, "postgresql"):
		clientConfig = getPostgreSQLConfig(ip, port, account)
	case sess.IsMongo():
		clientConfig = getMongoDBConfig(ip, port, account)
	default:
		return fmt.Errorf("unsupported protocol: %s", sess.Protocol)
	}

	logger.L().Info("Starting database client",
		zap.String("command", clientConfig.Command),
		zap.Strings("args", clientConfig.Args),
		zap.String("host", ip),
		zap.Int("port", port))

	// Create command and pseudo-terminal
	cmd := exec.CommandContext(sess.Gctx, clientConfig.Command, clientConfig.Args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		logger.L().Error("Failed to start database client with pty", zap.Error(err), zap.String("command", clientConfig.Command))
		return fmt.Errorf("failed to start %s: %w", clientConfig.Command, err)
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
		logger.L().Info("Database client process exited", zap.Error(err), zap.String("protocol", protocol))

		// Only send termination message if not already sent
		if atomic.CompareAndSwapInt32(&exitMessageSent, 0, 1) {
			// Send termination message
			terminationMsg := "\r\n\033[31mThe connection is closed!\033[0m\r\n"
			chs.OutBuf.WriteString(terminationMsg)
		}

		sess.Once.Do(func() {
			logger.L().Info("Closing AwayChan from database client monitor")
			close(chs.AwayChan)
		})
		return fmt.Errorf("database client process terminated: %w", err)
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
						isExitCmd := false
						for _, exitAlias := range clientConfig.ExitAliases {
							if strings.EqualFold(processCmd, exitAlias) {
								isExitCmd = true
								break
							}
						}

						if isExitCmd {
							// Send command to client for normal exit
							if _, err := ptmx.Write(buf[:n]); err != nil {
								return err
							}

							// Mark exit message as sent, but don't send it here
							atomic.StoreInt32(&exitMessageSent, 1)

							inputBuffer = ""
							continue
						}

						// Reset command buffer
						inputBuffer = ""
					}

					// Forward input to client
					if _, err := ptmx.Write(buf[:n]); err != nil {
						return err
					}
				}
			}
		}
	})

	// Goroutine 2: Read client output and send to OutChan
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
				// Adjust terminal size
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
