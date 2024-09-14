package guacd

import (
	"fmt"
	"strings"
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
	i := (&Instruction{}).Parse(string(p))
	return i.Opcode == "mouse" || i.Opcode == "key"
}
