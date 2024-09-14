package session

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/veops/go-ansiterm"
	mysql "github.com/veops/oneterm/db"
	"github.com/veops/oneterm/logger"
	"github.com/veops/oneterm/model"
	"go.uber.org/zap"
)

var (
	enterMarks = [][]byte{
		[]byte("\x1b[?1049h"),
		[]byte("\x1b[?1048h"),
		[]byte("\x1b[?1047h"),
		[]byte("\x1b[?47h"),
		[]byte("\x1b[?25l"),
	}

	exitMarks = [][]byte{
		[]byte("\x1b[?1049l"),
		[]byte("\x1b[?1048l"),
		[]byte("\x1b[?1047l"),
		[]byte("\x1b[?47l"),
		[]byte("\x1b[?25h"),
	}

	screenMarks = [][]byte{
		{0x1b, 0x5b, 0x4b, 0x0d, 0x0a},
		{0x1b, 0x5b, 0x34, 0x6c},
	}
)

func NewParser(sessionId string, w, h int) *Parser {
	screen := ansiterm.NewScreen(w, h)
	stream := ansiterm.InitByteStream(screen, false)
	stream.Attach(screen)
	p := &Parser{
		OutputStream: stream,
		isEdit:       false,
		first:        true,
	}
	return p
}

type Parser struct {
	OutputStream *ansiterm.ByteStream
	Input        []byte
	Output       []byte
	SessionId    string
	Cmds         []*model.Command
	first        bool
	prompt       string
	isEdit       bool
	lastCmd      string
	lastRes      string
}

func (p *Parser) AddInput(bs []byte) (cmd string, forbidden bool) {
	if p.first {
		p.GetOutput()
		p.first = false
	}
	p.Input = append(p.Input, bs...)
	if !bytes.HasSuffix(p.Input, []byte("\r")) {
		return
	}
	cmd = p.GetCmd()
	fmt.Println("----------------------cmd", cmd)
	p.Reset()
	filter := ""
	if filter, forbidden = p.IsForbidden(cmd); forbidden {
		cmd = filter
		return
	}
	p.lastCmd = cmd
	p.WriteDb()
	return
}

func (p *Parser) IsForbidden(cmd string) (string, bool) {
	for _, c := range p.Cmds {
		if c.IsRe {
			if c.Re.MatchString(cmd) {
				return fmt.Sprintf("Regex: %s", c.Cmd), true
			}
		} else {
			if strings.Contains(cmd, c.Cmd) {
				return c.Cmd, true
			}
		}
	}
	return "", false
}

func (p *Parser) WriteDb() {
	if p.lastCmd == "" {
		return
	}
	m := &model.SessionCmd{
		SessionId: p.SessionId,
		Cmd:       p.lastCmd,
		Result:    p.lastRes,
	}
	err := mysql.DB.Model(m).Create(m).Error
	if err != nil {
		logger.L().Error("write session cmd failed", zap.Error(err))
	}
}

func (p *Parser) AddOutput(bs []byte) {
	fmt.Println("-----------out", string(bs))
	if !p.isEdit {
		p.Output = append(p.Output, bs...)
		end := bytes.LastIndex(p.Output, []byte("\r"))
		if end < 0 {
			return
		}
		begin := end - 1
		for ; begin > 0; begin-- {
			if p.Output[begin] == '\r' {
				break
			}
		}
		if begin+1 > end-1 {
			return
		}
		p.prompt = string(p.Output[begin+1 : end-1])
	}
}

func (p *Parser) GetCmd() string {
	s := p.GetOutput()
	// TODO: some promot may change with its dir
	fmt.Println("============", s)
	fmt.Println("============", p.prompt)
	return strings.TrimPrefix(s, p.prompt)
}

func (p *Parser) Resize(w, h int) {
	p.OutputStream.Listener.Resize(w, h)
}

func (p *Parser) Reset() {
	p.OutputStream.Listener.Reset()
	p.Output = nil
	p.Input = nil
}

func (p *Parser) GetOutput() string {
	p.OutputStream.Feed(p.Output)

	res := parseOutput(p.OutputStream.Listener.Display())
	if len(res) == 0 {
		return ""
	}
	p.lastRes = res[len(res)-1]
	return p.lastRes
}

func parseOutput(data []string) (output []string) {
	for _, line := range data {
		if strings.TrimSpace(line) != "" {
			output = append(output, line)
		}
	}
	return output
}

func (p *Parser) State(b []byte) bool {
	if !p.isEdit && IsEditEnterMode(b) {
		if !isNewScreen(b) {
			p.isEdit = true
		}
	}
	if p.isEdit && IsEditExitMode(b) {
		p.Reset()
		p.isEdit = false
	}
	return p.isEdit
}

func isNewScreen(p []byte) bool {
	return matchMark(p, screenMarks)
}

func IsEditEnterMode(p []byte) bool {
	return matchMark(p, enterMarks)
}

func IsEditExitMode(p []byte) bool {
	return matchMark(p, exitMarks)
}

func matchMark(p []byte, marks [][]byte) bool {
	for _, item := range marks {
		if bytes.Contains(p, item) {
			return true
		}
	}
	return false
}
