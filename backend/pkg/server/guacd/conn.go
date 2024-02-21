package guacd

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/veops/oneterm/pkg/server/model"
)

const Version = "VERSION_1_5_0"

type Configuration struct {
	ConnectionId string
	Protocol     string
	Parameters   map[string]string
}

func NewConfiguration() (config *Configuration) {
	config = &Configuration{}
	config.Parameters = make(map[string]string)
	return config
}

type Tunnel struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	Uuid   string
	Config *Configuration
}

func NewTunnel(protocol string, asset *model.Asset, account *model.Account, gateway *model.Gateway) (t *Tunnel, err error) {
	ss := strings.Split(protocol, ":")
	protocol, port := ss[0], ss[1]

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", asset.Ip, port), time.Second*3)
	if err != nil {
		return
	}

	t = &Tunnel{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		Config: &Configuration{
			Protocol: protocol,
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
			defer t.conn.Close()
		}
	}()

	// select
	if err = t.WriteInstruction(NewInstruction("select", lo.Ternary(t.Config.ConnectionId == "", t.Config.Protocol, t.Config.ConnectionId))); err != nil {
		return
	}

	// args
	args, err := t.assert("args")
	if err != nil {
		return
	}
	parameters := make([]string, len(args.Args))
	for i, name := range args.Args {
		if strings.Contains(name, "VERSION") {
			parameters[i] = Version
			continue
		}
		parameters[i] = t.Config.Parameters[name]
	}

	// size audio ...
	if err = t.WriteInstruction(NewInstruction("size", t.Config.Parameters["width"], t.Config.Parameters["height"], t.Config.Parameters["dpi"])); err != nil {
		return
	}
	if err = t.WriteInstruction(NewInstruction("audio", "audio/L8", "audio/L16")); err != nil {
		return
	}
	if err = t.WriteInstruction(NewInstruction("video")); err != nil {
		return
	}
	if err = t.WriteInstruction(NewInstruction("image", "image/jpeg", "image/png", "image/webp")); err != nil {
		return
	}
	if err = t.WriteInstruction(NewInstruction("timezone", "Asia/Shanghai")); err != nil {
		return
	}

	// connect
	if err = t.WriteInstruction(NewInstruction("connect", parameters...)); err != nil {
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

	t.Uuid = ready.Args[0]

	return
}

func (t *Tunnel) WriteInstruction(instruction *Instruction) (err error) {
	_, err = t.writer.Write([]byte(instruction.String()))
	if err != nil {
		return
	}
	err = t.writer.Flush()
	if err != nil {
		return
	}
	return
}

func (t *Tunnel) ReadInstruction() (instruction *Instruction, err error) {
	data, err := t.reader.ReadBytes(Delimiter)
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
