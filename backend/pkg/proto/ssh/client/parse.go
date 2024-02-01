package client

import (
	"bytes"
	"sync"
)

type Parser struct {
	lock         sync.Mutex
	vimState     bool
	commandState bool
}

var (
	enterMarks = [][]byte{
		[]byte("\x1b[?1049h"),
		[]byte("\x1b[?1048h"),
		[]byte("\x1b[?1047h"),
		[]byte("\x1b[?47h"),
	}

	exitMarks = [][]byte{
		[]byte("\x1b[?1049l"),
		[]byte("\x1b[?1048l"),
		[]byte("\x1b[?1047l"),
		[]byte("\x1b[?47l"),
	}
	screenMarks = [][]byte{
		{0x1b, 0x5b, 0x4b, 0x0d, 0x0a},
		{0x1b, 0x5b, 0x34, 0x6c},
	}
)

func (p *Parser) State(b []byte) bool {
	if !p.vimState && IsEditEnterMode(b) {
		if !isNewScreen(b) {
			p.vimState = true
			p.commandState = false
		}
	}
	if p.vimState && IsEditExitMode(b) {
		p.vimState = false
		p.commandState = true
	}
	return p.vimState
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
