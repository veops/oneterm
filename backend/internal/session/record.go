package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
)

type Asciinema struct {
	file *os.File
	ts   time.Time
}

func NewAsciinema(id string, w, h int) (ret *Asciinema, err error) {
	f, err := os.Create(filepath.Join("/tmp/replay", fmt.Sprintf("%s.cast", id)))
	if err != nil {
		logger.L().Error("open cast failed", zap.String("id", id), zap.Error(err))
		return
	}
	ret = &Asciinema{file: f, ts: time.Now()}
	bs, _ := json.Marshal(map[string]any{
		"version":   2,
		"width":     w,
		"height":    h,
		"timestamp": ret.ts.Unix(),
		"title":     id,
		"env": map[string]any{
			"SHELL": "/bin/bash",
			"TERM":  "xterm-256color",
		},
	})
	ret.file.Write(append(bs, '\r', '\n'))
	return
}

func (a *Asciinema) Write(p []byte) {
	o := [3]any{}
	o[0] = float64(time.Now().UnixMicro()-a.ts.UnixMicro()) / 1_000_000
	o[1] = "o"
	o[2] = string(p)
	bs, _ := json.Marshal(o)
	a.file.Write(append(bs, '\r', '\n'))
}

func (a *Asciinema) Resize(w, h int) {
	r := [3]any{}
	r[0] = float64(time.Now().UnixMicro()-a.ts.UnixMicro()) / 1_000_000
	r[1] = "r"
	r[2] = fmt.Sprintf("%dx%d", w, h)
	bs, _ := json.Marshal(r)
	a.file.Write(append(bs, '\r', '\n'))
}
