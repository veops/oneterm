package guacd

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	internalDataOpcode = ""
	delimiter          = ';'
)

var (
	InternalOpcodeIns = []byte(fmt.Sprint(len(internalDataOpcode), ".", internalDataOpcode))
)

type Instruction struct {
	Opcode string
	Args   []string
	cache  string
}

func NewInstruction(opcode string, args ...string) *Instruction {
	return &Instruction{
		Opcode: opcode,
		Args:   args,
	}
}

func (i *Instruction) String() string {
	if len(i.cache) > 0 {
		return i.cache
	}

	i.cache = fmt.Sprintf("%d.%s", len(i.Opcode), i.Opcode)
	for _, value := range i.Args {
		i.cache += fmt.Sprintf(",%d.%s", len(value), value)
	}
	i.cache += string(delimiter)
	return i.cache
}

func (i *Instruction) Bytes() []byte {
	return []byte(i.String())
}

func (i *Instruction) Parse(content string) *Instruction {
	if strings.LastIndex(content, ";") > 0 {
		content = strings.TrimRight(content, ";")
	}
	elements := strings.Split(content, ",")

	var args = make([]string, len(elements))
	for i, e := range elements {
		ss := strings.Split(e, ".")
		if len(ss) < 2 {
			continue
		}
		args[i] = ss[1]
	}
	return NewInstruction(args[0], args[1:]...)
}

func IsActive(p []byte) bool {
	active := false
	for len(p) > 0 {
		opcode, size := instructionOpcode(p)
		if size == 0 {
			return false
		}
		active = active || bytes.Equal(opcode, []byte("mouse")) || bytes.Equal(opcode, []byte("key"))
		p = p[size:]
	}
	return active
}

func instructionOpcode(p []byte) (opcode []byte, size int) {
	for position := 0; position < len(p); {
		length, digits := 0, 0
		for position < len(p) && p[position] >= '0' && p[position] <= '9' {
			length = length*10 + int(p[position]-'0')
			if length > len(p) {
				return nil, 0
			}
			position++
			digits++
		}
		if digits == 0 || position >= len(p) || p[position] != '.' {
			return nil, 0
		}
		position++
		start := position
		for i := 0; i < length; i++ {
			if position >= len(p) {
				return nil, 0
			}
			width := 1
			if p[position] >= utf8.RuneSelf {
				_, width = utf8.DecodeRune(p[position:])
				if width == 1 {
					return nil, 0
				}
			}
			position += width
		}
		if opcode == nil {
			opcode = p[start:position]
		}
		if position >= len(p) {
			return nil, 0
		}
		switch p[position] {
		case ';':
			return opcode, position + 1
		case ',':
			position++
		default:
			return nil, 0
		}
	}
	return nil, 0
}
