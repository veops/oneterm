package session

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/samber/lo"
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
		SessionId:    sessionId,
		OutputStream: stream,
		isEdit:       false,
		isPrompt:     true,
	}
	return p
}

type Parser struct {
	OutputStream *ansiterm.ByteStream
	Input        []byte
	Output       []byte
	SessionId    string
	Cmds         []*model.Command
	isPrompt     bool
	prompt       string
	isEdit       bool
	curCmd       string
	lastCmd      string
	lastRes      string
	curRes       string
}

func (p *Parser) AddInput(bs []byte) (cmd string, forbidden bool) {
	if p.isPrompt && !p.isEdit {
		//TODO: may someone has empty ps1?
		if ps1 := p.GetOutput(); ps1 != "" {
			p.prompt = ps1
		}
		p.isPrompt = false
		p.WriteDb()
		p.lastCmd = ""
		p.lastRes = ""
	}
	p.Input = append(p.Input, bs...)
	if !bytes.HasSuffix(p.Input, []byte("\r")) {
		return
	}
	p.isPrompt = true
	p.curCmd = p.GetCmd()
	p.Reset()
	filter := ""
	if filter, forbidden = p.IsForbidden(p.curCmd); forbidden {
		cmd = filter
		return
	}
	p.lastCmd = p.curCmd
	return
}

func (p *Parser) IsForbidden(cmd string) (string, bool) {
	if p.isEdit || cmd == "" {
		return "", false
	}
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
		logger.L().Error("write session cmd failed", zap.Error(err), zap.Any("cmd", *m))
	}
}

func (p *Parser) AddOutput(bs []byte) {
	p.Output = append(p.Output, bs...)
}

func (p *Parser) GetCmd() string {
	s := p.GetOutput()
	// TODO: some promot may change with its dir
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

	res := p.OutputStream.Listener.Display()
	res = lo.DropRightWhile(res, func(item string) bool { return item == "" })
	ln := len(res)
	if ln == 0 {
		return ""
	}

	p.lastRes = ""
	if ln > 1 {
		p.lastRes = strings.Join(res[:ln-1], "\n")
	}
	p.curRes = res[ln-1]
	return p.curRes
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
