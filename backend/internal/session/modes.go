package session

import (
	"bytes"
	"strconv"
	"strings"
)

type terminalModes struct {
	escape    [64]byte
	length    int
	alternate uint8
	hidden    bool
	home      bool
	redraw    bool
}

func (m *terminalModes) feed(output []byte) bool {
	if m.length == 0 && bytes.IndexByte(output, 0x1b) < 0 {
		return m.alternate != 0 || m.hidden || m.redraw
	}
	for _, b := range output {
		if b == 0x1b {
			m.escape[0], m.length = b, 1
			continue
		}
		if m.length == 0 {
			continue
		}
		if m.length == len(m.escape) {
			m.length = 0
			continue
		}
		m.escape[m.length] = b
		m.length++
		if m.length == 2 && b != '[' {
			if b == 'c' {
				m.alternate, m.hidden, m.home, m.redraw = 0, false, false, false
			}
			m.length = 0
			continue
		}
		if m.length < 3 || b < 0x40 || b > 0x7e {
			continue
		}
		if b == 'H' {
			parameters := string(m.escape[2 : m.length-1])
			m.home = parameters == "" || parameters == "1;1" || parameters == "0;0"
		}
		if b == 'J' && m.home {
			parameters := string(m.escape[2 : m.length-1])
			if parameters == "" || parameters == "0" || parameters == "2" {
				m.redraw = true
			}
		}
		if m.length > 4 && m.escape[2] == '?' && (b == 'h' || b == 'l') {
			for _, parameter := range strings.Split(string(m.escape[3:m.length-1]), ";") {
				mode, _ := strconv.Atoi(parameter)
				var mask uint8
				switch mode {
				case 25:
					m.hidden = b == 'l'
					if !m.hidden && m.alternate == 0 {
						m.redraw = false
					}
				case 47:
					mask = 1
				case 1047:
					mask = 2
				case 1049:
					mask = 4
				}
				if b == 'h' {
					m.alternate |= mask
				} else {
					m.alternate &^= mask
					if mask != 0 && m.alternate == 0 {
						m.redraw = false
					}
				}
			}
		}
		m.length = 0
	}
	return m.alternate != 0 || m.hidden || m.redraw
}
