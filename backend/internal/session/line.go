package session

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type lineInput struct {
	text     []rune
	cursor   int
	escape   []byte
	pending  []byte
	paste    bool
	remote   bool
	overflow bool
}

func (l *lineInput) control(sequence string) {
	switch sequence {
	case "[D", "OD":
		l.cursor = max(0, l.cursor-1)
	case "[C", "OC":
		l.cursor = min(len(l.text), l.cursor+1)
	case "[H", "OH", "[1~", "[7~":
		l.cursor = 0
	case "[F", "OF", "[4~", "[8~":
		l.cursor = len(l.text)
	case "[3~":
		if l.cursor < len(l.text) {
			l.text = append(l.text[:l.cursor], l.text[l.cursor+1:]...)
		}
	case "[200~":
		l.paste = true
	case "[201~":
		l.paste = false
	default:
		l.remote = true
	}
}

func (l *lineInput) feed(p []byte) []string {
	if len(l.pending) > 0 {
		p = append(l.pending, p...)
		l.pending = nil
	}
	var commands []string
	for len(p) > 0 {
		b := p[0]
		if len(l.escape) > 0 {
			if b == '\r' || b == '\n' {
				l.escape = nil
				l.remote = true
			} else {
				l.escape = append(l.escape, b)
				p = p[1:]
				if len(l.escape) == 2 && (b == '[' || b == 'O') {
					continue
				}
				if len(l.escape) == 2 {
					l.escape = nil
					l.remote = true
					continue
				}
				if len(l.escape) > 2 && b >= 0x40 && b <= 0x7e || len(l.escape) > 32 {
					l.control(string(l.escape[1:]))
					l.escape = nil
				}
				continue
			}
		}
		switch b {
		case 0x1b:
			l.escape = []byte{b}
		case '\r', '\n':
			if l.paste {
				if len(l.text) < 65536 {
					l.text = append(l.text, '\n')
					l.cursor = len(l.text)
				} else {
					l.overflow = true
				}
			} else {
				commands = append(commands, strings.TrimSpace(string(l.text)))
				l.text, l.cursor = nil, 0
			}
		case 0x7f, 0x08:
			if l.cursor > 0 {
				l.text = append(l.text[:l.cursor-1], l.text[l.cursor:]...)
				l.cursor--
			}
		case 0x01:
			l.cursor = 0
		case 0x05:
			l.cursor = len(l.text)
		case 0x02:
			l.cursor = max(0, l.cursor-1)
		case 0x06:
			l.cursor = min(len(l.text), l.cursor+1)
		case 0x04:
			if l.cursor < len(l.text) {
				l.text = append(l.text[:l.cursor], l.text[l.cursor+1:]...)
			}
		case 0x0b:
			l.text = l.text[:l.cursor]
		case 0x15:
			l.text = l.text[l.cursor:]
			l.cursor = 0
		case 0x03:
			l.text, l.cursor, l.remote, l.overflow = nil, 0, false, false
		case 0x17:
			end := l.cursor
			for l.cursor > 0 && unicode.IsSpace(l.text[l.cursor-1]) {
				l.cursor--
			}
			for l.cursor > 0 && !unicode.IsSpace(l.text[l.cursor-1]) {
				l.cursor--
			}
			l.text = append(l.text[:l.cursor], l.text[end:]...)
		case '\t':
			l.remote = true
		case 0x0e, 0x10, 0x12, 0x16, 0x19:
			l.remote = true
		default:
			if b >= 0x20 {
				if !utf8.FullRune(p) {
					l.pending = append(l.pending, p...)
					return commands
				}
				r, n := utf8.DecodeRune(p)
				if len(l.text) < 65536 {
					l.text = append(l.text, 0)
					copy(l.text[l.cursor+1:], l.text[l.cursor:])
					l.text[l.cursor] = r
					l.cursor++
				} else {
					l.overflow = true
				}
				p = p[n:]
				continue
			}
		}
		p = p[1:]
	}
	return commands
}
