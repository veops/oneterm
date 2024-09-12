package guacd

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/conf"
	ggateway "github.com/veops/oneterm/gateway"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
)

const (
	VERSION          = "VERSION_1_5_0"
	RECORDING_PATH   = "/replay"
	CREATE_RECORDING = "true"
	IGNORE_CERT      = "true"
)

type Configuration struct {
	Protocol   string
	Parameters map[string]string
}

func NewConfiguration() (config *Configuration) {
	config = &Configuration{}
	config.Parameters = make(map[string]string)
	return config
}

type Tunnel struct {
	SessionId    string
	ConnectionId string
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	Config       *Configuration
	gw           *ggateway.GatewayTunnel
}

func NewTunnel(connectionId, sessionId string, w, h, dpi int, protocol string, asset *model.Asset, account *model.Account, gateway *model.Gateway) (t *Tunnel, err error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", conf.Cfg.Guacd.Host, conf.Cfg.Guacd.Port), time.Second*3)
	if err != nil {
		return
	}
	ss := strings.Split(protocol, ":")
	protocol, port := ss[0], ss[1]
	cfg := model.GlobalConfig.Load()
	t = &Tunnel{
		conn:         conn,
		reader:       bufio.NewReader(conn),
		writer:       bufio.NewWriter(conn),
		ConnectionId: connectionId,
		Config: &Configuration{
			Protocol: protocol,
			Parameters: lo.TernaryF(
				connectionId == "",
				func() map[string]string {
					return map[string]string{
						"version":               VERSION,
						"recording-path":        RECORDING_PATH,
						"create-recording-path": CREATE_RECORDING,
						"ignore-cert":           IGNORE_CERT,
						"width":                 cast.ToString(w),
						"height":                cast.ToString(h),
						"dpi":                   cast.ToString(dpi),
						"scheme":                protocol,
						"hostname":              asset.Ip,
						"port":                  port,
						"username":              account.Account,
						"password":              account.Password,
						"disable-copy":          cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), !cfg.RdpConfig.Copy, !cfg.VncConfig.Copy)),
						"disable-paste":         cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), !cfg.RdpConfig.Paste, !cfg.VncConfig.Paste)),
					}
				}, func() map[string]string {
					return map[string]string{
						"width":     cast.ToString(w),
						"height":    cast.ToString(h),
						"dpi":       cast.ToString(dpi),
						"read-only": "true",
					}
				}),
		},
	}
	if t.ConnectionId == "" {
		t.SessionId = sessionId
		t.Config.Parameters["recording-name"] = t.SessionId
	}
	if gateway != nil && gateway.Id != 0 && t.ConnectionId == "" {
		t.gw, err = ggateway.GetGatewayManager().Open(t.SessionId, asset.Ip, cast.ToInt(port), gateway)
		if err != nil {
			return t, err
		}
		t.Config.Parameters["hostname"] = "localhost"
		t.Config.Parameters["port"] = cast.ToString(t.gw.LocalPort)
	}

	err = t.handshake()

	return
}

// handshake
//
//	https://guacamole.apache.org/doc/gug/guacamole-protocol.html#handshake-phase
func (t *Tunnel) handshake() (err error) {
	defer func() {
		if err != nil {
			t.conn.Close()
		}
	}()

	// select
	if _, err = t.WriteInstruction(NewInstruction("select", lo.Ternary(t.ConnectionId == "", t.Config.Protocol, t.ConnectionId))); err != nil {
		return
	}

	// args
	args, err := t.assert("args")
	if err != nil {
		return
	}
	parameters := make([]string, len(args.Args))
	for i, name := range args.Args {
		parameters[i] = t.Config.Parameters[name]
	}

	// size audio video image
	if _, err = t.WriteInstruction(NewInstruction("size", t.Config.Parameters["width"], t.Config.Parameters["height"], t.Config.Parameters["dpi"])); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("audio", "audio/L8")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("video")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("image", "image/jpeg", "image/png", "image/webp")); err != nil {
		return
	}

	// connect
	if _, err = t.WriteInstruction(NewInstruction("connect", parameters...)); err != nil {
		return
	}

	// ready
	ready, err := t.assert("ready")
	if err != nil {
		return
	}

	if len(ready.Args) == 0 {
		err = fmt.Errorf("empty connection id")
		return
	}

	t.ConnectionId = ready.Args[0]

	return
}

func (t *Tunnel) Write(p []byte) (n int, err error) {
	if t == nil || t.writer == nil {
		return
	}
	n, err = t.writer.Write(p)
	if err != nil {
		return
	}
	err = t.writer.Flush()
	return
}

func (t *Tunnel) WriteInstruction(instruction *Instruction) (n int, err error) {
	n, err = t.Write(instruction.Bytes())
	return
}

func (t *Tunnel) Read() (p []byte, err error) {
	p, err = t.reader.ReadBytes(delimiter)
	return
}

func (t *Tunnel) ReadInstruction() (instruction *Instruction, err error) {
	data, err := t.Read()
	if err != nil {
		return
	}

	instruction = (&Instruction{}).Parse(string(data))

	return
}

func (t *Tunnel) assert(opcode string) (instruction *Instruction, err error) {
	instruction, err = t.ReadInstruction()
	if err != nil {
		return
	}

	if opcode != instruction.Opcode {
		err = fmt.Errorf(`expect instruction "%s" but got "%s"`, opcode, instruction.Opcode)
	}

	return
}

func (t *Tunnel) Close() {
	ggateway.GetGatewayManager().Close(t.SessionId)
}

func (t *Tunnel) Disconnect() {
	logger.L().Debug("client disconnect")
	t.WriteInstruction(NewInstruction("disconnect"))
}
