package protocols

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/logger"
)

// Telnet protocol constants
const (
	IAC  = byte(255) // Interpret As Command
	WILL = byte(251)
	WONT = byte(252)
	DO   = byte(253)
	DONT = byte(254)
	SB   = byte(250) // Sub-negotiation Begin
	SE   = byte(240) // Sub-negotiation End
	GA   = byte(249) // Go Ahead

	// Telnet options
	OPT_ECHO           = byte(1)  // Echo
	OPT_SUPPRESS_GA    = byte(3)  // Suppress Go Ahead
	OPT_TERMINAL_TYPE  = byte(24) // Terminal Type
	OPT_NAWS           = byte(31) // Negotiate About Window Size
	OPT_TERMINAL_SPEED = byte(32) // Terminal Speed
	OPT_LINEMODE       = byte(34) // Linemode
	OPT_NEW_ENVIRON    = byte(39) // New Environment
)

// ConnectTelnet establishes a connection to a Telnet server and handles the session
// It performs authentication, sets up the environment, and manages data flow
// between the client and server until the session ends.
func ConnectTelnet(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			logger.L().Error("telnet connection error", zap.Error(err))
			chs.ErrChan <- err
		}
	}()

	logger.L().Info("starting telnet connection",
		zap.String("sessionId", sess.SessionId),
		zap.String("asset", asset.Name),
		zap.String("ip", asset.Ip))

	// Establish connection through tunneling
	ip, port, err := tunneling.Proxy(false, sess.SessionId, "telnet", asset, gateway)
	if err != nil {
		logger.L().Error("telnet tunneling failed", zap.Error(err))
		return
	}

	// Connect to the telnet server
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 10*time.Second)
	if err != nil {
		logger.L().Error("telnet dial failed", zap.Error(err))
		return
	}
	defer conn.Close()

	// Setup authentication control mechanisms
	authDone := make(chan bool, 1)
	authErr := make(chan error, 1)
	var prompt strings.Builder
	var promptMutex sync.Mutex
	loginSent := false
	passwordSent := false
	terminalTypeSent := false

	// Authentication handler goroutine
	// Monitors server responses for login/password prompts and responds accordingly
	go func() {
		timeoutChan := time.After(5 * time.Second)

		for {
			select {
			case <-timeoutChan:
				authDone <- true
				return
			default:
				buf := make([]byte, 1024)
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, err := conn.Read(buf)
				conn.SetReadDeadline(time.Time{})

				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}

					if err == io.EOF {
						authErr <- fmt.Errorf("connection closed during authentication")
						return
					}

					logger.L().Error("read error during authentication", zap.Error(err))
					authErr <- err
					return
				}

				if n > 0 {
					// Process telnet protocol commands and extract actual data
					processed := processTelnetData(buf[:n], conn)
					if len(processed) > 0 {
						chs.OutChan <- processed

						// Update prompt buffer to detect login/password prompts
						promptMutex.Lock()
						prompt.Write(processed)
						if prompt.Len() > 200 {
							promptStr := prompt.String()
							prompt.Reset()
							prompt.WriteString(promptStr[len(promptStr)-200:])
						}
						promptStr := strings.ToLower(prompt.String())
						promptMutex.Unlock()

						// Check for username prompt
						if !loginSent && (strings.Contains(promptStr, "login") ||
							strings.Contains(promptStr, "username") ||
							strings.Contains(promptStr, "account")) {

							time.Sleep(300 * time.Millisecond)

							_, err := conn.Write([]byte(account.Account + "\r\n"))
							if err != nil {
								logger.L().Error("send username failed", zap.Error(err))
								authErr <- err
								return
							}

							loginSent = true

							promptMutex.Lock()
							prompt.Reset()
							promptMutex.Unlock()
						}

						// Check for password prompt
						if loginSent && !passwordSent && (strings.Contains(promptStr, "password") ||
							strings.Contains(promptStr, "pass:")) {

							time.Sleep(300 * time.Millisecond)

							_, err := conn.Write([]byte(account.Password + "\r\n"))
							if err != nil {
								logger.L().Error("send password failed", zap.Error(err))
								authErr <- err
								return
							}

							passwordSent = true

							// Give server time to process login before moving on
							go func() {
								time.Sleep(1 * time.Second)
								authDone <- true
							}()
						}

						// Detect successful login by checking for command prompt characters
						if (loginSent && passwordSent) ||
							strings.Contains(promptStr, "$") ||
							strings.Contains(promptStr, "#") ||
							strings.Contains(promptStr, ">") {

							if !terminalTypeSent {
								setTerminalType(conn)
								terminalTypeSent = true
							}

							authDone <- true
							return
						}
					}
				}
			}
		}
	}()

	// Wait for authentication to complete or timeout
	select {
	case <-authDone:
		setupEnvironment(conn)
	case err := <-authErr:
		logger.L().Error("telnet authentication failed", zap.Error(err))
		return err
	case <-time.After(10 * time.Second):
		// Fallback: proceed even if authentication times out
		setupEnvironment(conn)
	}

	// Data flow from client to server
	// Reads from input pipe and writes to telnet connection
	sess.G.Go(func() error {
		buf := make([]byte, 1024)
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				n, err := chs.Rin.Read(buf)
				if err != nil {
					if err == io.EOF {
						continue
					}
					if err.Error() == "io: read/write on closed pipe" {
						return nil // Normal exit condition, not an error
					}
					logger.L().Error("read from input pipe failed", zap.Error(err))
					return err
				}

				if n > 0 {
					_, err = conn.Write(buf[:n])
					if err != nil {
						logger.L().Error("write to telnet failed", zap.Error(err))
						return err
					}
				}
			}
		}
	})

	// Data flow from server to client
	// Reads from telnet connection, processes telnet protocol, and sends to output channel
	sess.G.Go(func() error {
		buf := make([]byte, 8192)
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				// Use deadline to avoid blocking indefinitely
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, err := conn.Read(buf)
				conn.SetReadDeadline(time.Time{})

				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					if err == io.EOF {
						logger.L().Info("telnet connection closed by server")
						return fmt.Errorf("telnet connection closed")
					}
					if strings.Contains(err.Error(), "use of closed network connection") {
						return nil // Normal connection close, not an error
					}
					logger.L().Error("read from telnet failed", zap.Error(err))
					return err
				}

				if n > 0 {
					data := processTelnetData(buf[:n], conn)
					if len(data) > 0 {
						chs.OutChan <- data
					}
				}
			}
		}
	})

	// Signal successful connection
	chs.ErrChan <- nil
	// Wait for all goroutines to complete
	err = sess.G.Wait()

	return nil
}

// setTerminalType configures the terminal type and dimensions
// This helps ensure proper display of text and ANSI sequences
func setTerminalType(conn net.Conn) {
	_, err := conn.Write([]byte("export TERM=xterm-256color\r\n"))
	if err != nil {
		logger.L().Error("failed to set TERM environment variable", zap.Error(err))
	}

	_, err = conn.Write([]byte("export LINES=24 COLUMNS=80\r\n"))
	if err != nil {
		logger.L().Error("failed to set terminal dimensions", zap.Error(err))
	}

	_, err = conn.Write([]byte("stty rows 24 columns 80\r\n"))
	if err != nil {
		logger.L().Error("failed to set stty configuration", zap.Error(err))
	}
}

// setupEnvironment configures the shell environment for optimal telnet operation
// Disables features that could interfere with terminal display or functionality
func setupEnvironment(conn net.Conn) {
	time.Sleep(500 * time.Millisecond)
	setTerminalType(conn)

	_, err := conn.Write([]byte("set +o histappend 2>/dev/null || true\r\n"))
	if err != nil {
		logger.L().Error("failed to configure shell history", zap.Error(err))
	}

	_, err = conn.Write([]byte("unalias -a 2>/dev/null || true\r\n"))
	if err != nil {
		logger.L().Error("failed to unalias commands", zap.Error(err))
	}

	_, err = conn.Write([]byte("clear 2>/dev/null || echo -e '\\033c'\r\n"))
	if err != nil {
		logger.L().Error("failed to clear screen", zap.Error(err))
	}
}

// processTelnetData handles telnet protocol control sequences
// It extracts actual data from the stream by filtering out protocol commands
// and responding to negotiation requests appropriately
func processTelnetData(data []byte, conn net.Conn) []byte {
	if len(data) == 0 {
		return data
	}

	processed := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] == IAC && i+1 < len(data) {
			i++ // Skip IAC byte
			if i >= len(data) {
				break
			}

			cmd := data[i]

			if cmd >= WILL && cmd <= DONT && i+1 < len(data) {
				// Handle negotiation commands (WILL/WONT/DO/DONT)
				opt := data[i+1]

				if cmd == WILL || cmd == DO {
					// Respond to capability negotiations
					var response []byte
					if cmd == WILL {
						response = []byte{IAC, DONT, opt} // Reject server's offer
					} else {
						response = []byte{IAC, WONT, opt} // Reject server's request
					}

					if _, err := conn.Write(response); err != nil {
						logger.L().Error("failed to send negotiation response", zap.Error(err))
					}
				}

				i++ // Skip option byte
			} else if cmd == SB {
				// Handle subnegotiation: skip until IAC SE
				i++
				for i < len(data)-1 {
					if data[i] == IAC && data[i+1] == SE {
						i++
						break
					}
					i++
				}
			}
			// Skip control sequence in output data
		} else {
			// Regular data byte, add to processed output
			processed = append(processed, data[i])
		}
	}

	return processed
}
