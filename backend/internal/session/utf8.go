package session

import (
	"bytes"
	"unicode/utf8"
)

func terminalText(pending, data []byte) ([]byte, []byte) {
	if len(pending) > 0 {
		data = append(pending, data...)
	}
	end := len(data)
	if end > 0 && !utf8.Valid(data) {
		start := end - 1
		for start > 0 && end-start < utf8.UTFMax && data[start]&0xc0 == 0x80 {
			start--
		}
		if !utf8.FullRune(data[start:]) {
			end = start
		}
	}
	var rest []byte
	if end < len(data) {
		rest = append([]byte(nil), data[end:]...)
	}
	return bytes.ToValidUTF8(data[:end], []byte("\ufffd")), rest
}
