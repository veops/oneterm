package session

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/samber/lo"
	"github.com/veops/go-ansiterm"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	dbpkg "github.com/veops/oneterm/pkg/db"
	"github.com/veops/oneterm/pkg/logger"
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
		mu:           &sync.Mutex{},
	}
	return p
}

type Parser struct {
	OutputStream *ansiterm.ByteStream
	Input        []byte
	Output       []byte
	SessionId    string
	Protocol     string
	Cmds         []*model.Command
	isPrompt     bool
	prompt       string
	isEdit       bool
	curCmd       string
	lastCmd      string
	lastRes      string
	curRes       string
	mu           *sync.Mutex
}

func (p *Parser) AddInput(bs []byte) (cmd string, forbidden bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isPrompt && !p.isEdit {
		//TODO: may someone has empty ps1?
		if ps1 := p.getOutputLocked(); ps1 != "" {
			p.prompt = ps1
		}
		p.isPrompt = false
		p.WriteDb()
		p.lastCmd = ""
		p.lastRes = ""
	}

	// Track command input, directly use curCmd field
	if len(bs) > 0 {
		if bytes.Equal(bs, []byte("\r")) {
			// Command ends, keep curCmd unchanged
		} else if len(bs) == 1 && (bs[0] == '\b' || bs[0] == 127) { // Backspace key
			if len(p.curCmd) > 0 {
				p.curCmd = p.curCmd[:len(p.curCmd)-1]
			}
		} else {
			// Check if all characters are printable and record input
			input := string(bs)
			allPrintable := true
			for _, ch := range input {
				if ch < 32 || ch > 126 {
					allPrintable = false
					break
				}
			}
			if allPrintable {
				p.curCmd += input
			}
		}
	}

	p.Input = append(p.Input, bs...)
	if !bytes.HasSuffix(p.Input, []byte("\r")) {
		return
	}

	p.isPrompt = true
	// Save current command
	currentCmd := strings.TrimSpace(p.curCmd)
	// Reset command buffer
	p.curCmd = ""
	p.resetLocked()

	filter := ""
	if filter, forbidden = p.IsForbidden(currentCmd); forbidden {
		cmd = filter
		return
	}
	p.lastCmd = currentCmd
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
	if p.lastCmd == "" || strings.TrimSpace(p.lastCmd) == "" {
		return
	}
	m := &model.SessionCmd{
		SessionId: p.SessionId,
		Cmd:       p.lastCmd,
		Result:    p.lastRes,
	}
	err := dbpkg.DB.Model(m).Create(m).Error
	if err != nil {
		logger.L().Error("write session cmd failed", zap.Error(err), zap.Any("cmd", *m))
	}
}

func (p *Parser) Close(prompt string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if prompt == "" {
		prompt = p.prompt
	}
	p.AddOutputLocked([]byte("\r\n" + prompt))
	p.AddInputLocked([]byte("\r"))
}

func (p *Parser) AddOutput(bs []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.AddOutputLocked(bs)
}

func (p *Parser) AddOutputLocked(bs []byte) {
	p.Output = append(p.Output, bs...)
}

func (p *Parser) AddInputLocked(bs []byte) {
	p.Input = append(p.Input, bs...)
}

func (p *Parser) GetCmd() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.getCmdLocked()
}

func (p *Parser) getCmdLocked() string {
	// If the current command being built is not empty, return it
	if p.curCmd != "" {
		return p.curCmd
	}

	// Otherwise extract from output
	s := p.getOutputLocked()
	// TODO: some promot may change with its dir
	return strings.TrimPrefix(s, p.prompt)
}

func (p *Parser) Resize(w, h int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.OutputStream.Listener.Resize(w, h)
}

func (p *Parser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.resetLocked()
}

func (p *Parser) resetLocked() {
	p.OutputStream.Listener.Reset()
	p.Output = nil
	p.Input = nil
}

func (p *Parser) GetOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.getOutputLocked()
}

func (p *Parser) getOutputLocked() string {
	p.OutputStream.Feed(p.Output)

	res := p.OutputStream.Listener.Display()
	res = lo.DropRightWhile(res, func(item string) bool { return item == "" })
	ln := len(res)
	if ln == 0 {
		return ""
	}

	p.lastRes = ""
	if ln > 1 {
		// Process result based on protocol type
		if strings.HasPrefix(p.Protocol, "ssh") {
			// For SSH sessions, keep the original logic, retain all lines
			p.lastRes = strings.Join(res[:ln-1], "\n")
		} else {
			// For non-SSH sessions (like Redis, MySQL), remove first and last line
			// to avoid recording the prompt as part of the command result
			startIdx := 1
			endIdx := ln - 1

			// Ensure there are enough lines to process
			if ln > 2 {
				p.lastRes = strings.Join(res[startIdx:endIdx], "\n")
			}
		}
	}
	p.curRes = res[ln-1]
	return p.curRes
}

func (p *Parser) State(b []byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isEdit && IsEditEnterMode(b) {
		if !isNewScreen(b) {
			p.isEdit = true
		}
	}
	if p.isEdit && IsEditExitMode(b) {
		p.resetLocked()
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
