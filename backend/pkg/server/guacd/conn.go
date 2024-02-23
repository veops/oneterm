package guacd

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/server/model"
	"github.com/veops/oneterm/pkg/util"
)

const Version = "VERSION_1_5_0"

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
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	ConnectionId string
	Config       *Configuration
}

func NewTunnel(connectionId string, w, h, dpi int, protocol string, asset *model.Asset, account *model.Account, gateway *model.Gateway) (t *Tunnel, err error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", conf.Cfg.Guacd.Ip, conf.Cfg.Guacd.Port), time.Second*3)
	if err != nil {
		return
	}
	ss := strings.Split(protocol, ":")
	protocol, port := ss[0], ss[1]
	t = &Tunnel{
		conn:         conn,
		reader:       bufio.NewReader(conn),
		writer:       bufio.NewWriter(conn),
		ConnectionId: connectionId,
		Config: &Configuration{
			Protocol: protocol,
			Parameters: map[string]string{
				"width":       cast.ToString(w),
				"height":      cast.ToString(h),
				"dpi":         cast.ToString(dpi),
				"scheme":      protocol,
				"hostname":    asset.Ip,
				"port":        port,
				"ignore-cert": "true",
				"security":    "",
				"username":    account.Account,
				"password":    util.DecryptAES(account.Password),
			},
		},
	}

	return
}

// Handshake
//
//	https://guacamole.apache.org/doc/gug/guacamole-protocol.html#handshake-phase
func (t *Tunnel) Handshake() (err error) {
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
	if _, err = t.WriteInstruction(NewInstruction("audio", "audio/L16", "rate=44100", "channels=2")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("video")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("image")); err != nil {
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
	n, err = t.writer.Write(p)
	if err != nil {
		return
	}
	err = t.writer.Flush()
	return
}

func (t *Tunnel) WriteInstruction(instruction *Instruction) (n int, err error) {
	n, err = t.Write([]byte(instruction.String()))
	return
}

func (t *Tunnel) Read() (p []byte, err error) {
	p, err = t.reader.ReadBytes(Delimiter)
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
