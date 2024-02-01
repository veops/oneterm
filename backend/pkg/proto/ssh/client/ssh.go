package client

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	gossh "github.com/gliderlabs/ssh"
	"github.com/google/uuid"
	gssh "golang.org/x/crypto/ssh"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/record"
	"github.com/veops/oneterm/pkg/server/model"
)

type Connection struct {
	Session *gssh.Session
	Stdin   io.Writer
	Stdout  io.Reader

	SessionId string
	Record    record.Record
	Commands  []byte
	AssetId   int
	AccountId int
	Gateway   *model.Gateway

	Parser *Parser

	GateWayCloseChan chan struct{}
	Exit             chan struct{}
}

type GatewayClient struct {
	client     *gssh.Client
	targetAddr string
}

var (
	GatewayListener    net.Listener
	GatewayConnections sync.Map
)

func NewSSHClientConfig(user string, account *model.Account) (*gssh.ClientConfig, error) {
	am, er := authMethod(account)
	if er != nil {
		return nil, er
	}
	sshConfig := &gssh.ClientConfig{
		Timeout: time.Second * 5,
		User:    user,
		Auth: []gssh.AuthMethod{
			am,
		},
		HostKeyCallback: gssh.InsecureIgnoreHostKey(), // 不验证服务器的HostKey
	}
	return sshConfig, nil
}

func authMethod(account *model.Account) (gssh.AuthMethod, error) {
	switch account.AccountType {
	case model.AUTHMETHOD_PASSWORD:
		return gssh.Password(account.Password), nil
	case model.AUTHMETHOD_PUBLICKEY:
		if account.Phrase == "" {
			pk, err := gssh.ParsePrivateKey([]byte(account.Pk))
			if err != nil {
				return nil, err
			}
			return gssh.PublicKeys(pk), nil
		} else {
			pk, err := gssh.ParsePrivateKeyWithPassphrase([]byte(account.Pk), []byte(account.Phrase))
			if err != nil {
				return nil, err
			}
			return gssh.PublicKeys(pk), nil
		}
	default:
		return nil, fmt.Errorf("invalid authmethod %d", account.AccountType)
	}
}

// publicKeyBytes
// path: ~/.ssh/id_ed25519
//func publicKeyBytes(path string) error {
//	pbk, err := os.ReadFile(path)
//	publicKey, err := gossh.ParsePublicKey(pbk)
//	if err != nil {
//		return err
//	}
//	gossh.PublicKeyAuth(func(ctx gossh.Context, key gossh.PublicKey) bool {
//		return gossh.KeysEqual(key, publicKey)
//	})
//	return nil
//}

func NewSShSession(con *gssh.Client, pty gossh.Pty, gatewayCloseChan chan struct{}) (conn *Connection, err error) {
	sess, er := con.NewSession()
	if er != nil {
		err = er
		return
	}
	modes := gssh.TerminalModes{
		gssh.ECHO:          1,
		gssh.TTY_OP_ISPEED: 14400,
		gssh.TTY_OP_OSPEED: 14400,
	}
	if err = sess.RequestPty("xterm", pty.Window.Height, pty.Window.Width, modes); err != nil {
		return
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return
	}
	if err := sess.Shell(); err != nil {
		_ = sess.Close()
	}
	conn = &Connection{
		Stdin:            stdin,
		Stdout:           stdout,
		Session:          sess,
		SessionId:        uuid.NewString(),
		GateWayCloseChan: gatewayCloseChan,
		Exit:             make(chan struct{}),
	}

	conn.Record, err = record.NewAsciinema(conn.SessionId, pty)
	conn.Parser = &Parser{
		vimState:     false,
		commandState: true,
		lock:         sync.Mutex{},
	}
	return
}

// NewSShClient1
// =====================================================do not edit=============================================
func NewSShClient(addr string, account *model.Account, gateway *model.Gateway) (cli *gssh.Client, gatewayCloseChan chan struct{}, err error) {
	sshConf, err := NewSSHClientConfig(account.Account, account)
	if err != nil {
		return
	}

	tmp := strings.Split(strings.TrimSpace(addr), ":")
	if len(tmp) != 2 {
		tmp = append(tmp, "22")
	}
	addr = strings.Join(tmp, ":")

	if gateway != nil {
		gatewayCloseChan = make(chan struct{})
		gatewayConf, er := NewSSHClientConfig(gateway.Account,
			&model.Account{AccountType: gateway.AccountType, Account: gateway.Account,
				Password: gateway.Password, Pk: gateway.Pk, Phrase: gateway.Phrase})
		if er != nil {
			err = fmt.Errorf("gateway is not available %w", er)
			return
		}
		gatewayCli, er := gssh.Dial("tcp", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port), gatewayConf)
		if er != nil {
			err = fmt.Errorf("gateway is not available %w", er)
			return
		}

		//hostname, er := os.Hostname()
		//if er != nil {
		//	err = fmt.Errorf("gateway is not available %w", er)
		//	return
		//}
		targetAddr := addr
		//addr = fmt.Sprintf("%s:%d", hostname, port)
		port, er := GetAvailablePort()
		addr = fmt.Sprintf("127.0.0.1:%d", port)
		listener, er := net.Listen("tcp", addr)
		if er != nil {
			err = fmt.Errorf("gateway is not available %w", er)
			return
		}

		var accept bool
		go func() {
			for {
				select {
				case <-gatewayCloseChan:
					return
				default:
					if accept {
						continue
					}
					lc, err := listener.Accept()
					if err != nil {
						return
					}
					gatewayConn, err := gatewayCli.Dial("tcp", targetAddr)
					if err != nil {
						return
					}

					go func() {
						_, _ = io.Copy(lc, gatewayConn)
					}()
					go func() {
						_, _ = io.Copy(gatewayConn, lc)
					}()
					accept = true
				}
			}
		}()
	}
	cli, err = gssh.Dial("tcp", addr, sshConf)
	return
}

func ResizeSshClient(sess *gssh.Session, h, w int) {
	err := sess.WindowChange(h, w)
	if err != nil {
		logger.L.Warn(err.Error())
		return
	}
}
func GetAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer func(l *net.TCPListener) {
		_ = l.Close()
	}(l)
	return l.Addr().(*net.TCPAddr).Port, nil
}

func AcquireGatewayListener() (string, error) {
	if GatewayListener == nil {
		port, err := GetAvailablePort()
		if err != nil {
			return "", fmt.Errorf("get available port failed:%s", err.Error())
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, er := net.Listen("tcp", addr)
		if er != nil {
			return "", fmt.Errorf("listen tcp %s failed: %s", addr, er.Error())
		}
		GatewayListener = listener
		ListenGateway()
	}
	return GatewayListener.Addr().String(), nil
}

// func NewSShClient1(addr string, account *model.Account, gateway *model.Gateway) (cli *gssh.Client, gatewayCloseChan chan struct{}, err error) {
// 	password, pubkey := account.Password, ""
// 	if account.AccountType == model.AUTHMETHOD_PUBLICKEY {
// 		password, pubkey = pubkey, password
// 	}
// 	sshConf, err := NewSSHClientConfig(account.Account, password, pubkey)
// 	if err != nil {
// 		return
// 	}

// 	tmp := strings.Split(strings.TrimSpace(addr), ":")
// 	if len(tmp) != 2 {
// 		tmp = append(tmp, "22")
// 	}
// 	addr = strings.Join(tmp, ":")

// 	if gateway != nil {
// 		gatewayCloseChan = make(chan struct{})
// 		gatewayConf, er := NewSSHClientConfig(gateway.Account, gateway.Password, "")
// 		if er != nil {
// 			err = fmt.Errorf("gateway is not available %w", er)
// 			return
// 		}

// 		gatewayCli, er := gssh.Dial("tcp", fmt.Sprintf("%s:%d", gateway.Host, gateway.Port), gatewayConf)
// 		if er != nil {
// 			err = fmt.Errorf("gateway is not available %w", er)
// 			return
// 		}

// 		if gatewayAddr, er := AcquireGatewayListener(); er != nil {
// 			err = er
// 			return
// 		} else {
// 			fmt.Println("dial.........", gatewayAddr, sshConf)
// 			//c, er := net.DialTimeout("tcp", gatewayAddr, time.Second*5)
// 			//fmt.Println(c, er)
// 			cli, err = gssh.Dial("tcp", gatewayAddr, sshConf)
// 			if err != nil {
// 				return
// 			}
// 			fmt.Println("store.......")
// 			GatewayConnections.Store(cli.LocalAddr().String(), GatewayClient{client: gatewayCli, targetAddr: addr})
// 			fmt.Println("endd dial...", err)
// 		}

// 	} else {
// 		cli, err = gssh.Dial("tcp", addr, sshConf)
// 	}
// 	return
// }

func ListenGateway() {

	go func() {
		for {
			conn, err := GatewayListener.Accept()
			if err != nil {
				logger.L.Warn(err.Error())
				return
			}
			if v, ok := GatewayConnections.Load(conn.RemoteAddr().String()); ok {
				cli := v.(GatewayClient)
				gatewayConn, err := cli.client.Dial("tcp", cli.targetAddr)
				if err != nil {
					logger.L.Warn(err.Error())
					break
				}
				go func() {
					_, _ = io.Copy(conn, gatewayConn)
				}()
				go func() {
					_, _ = io.Copy(gatewayConn, conn)
				}()
			}
		}
	}()
}
