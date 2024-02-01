package record

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gossh "github.com/gliderlabs/ssh"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/api"
	"github.com/veops/oneterm/pkg/proto/ssh/config"
)

type Asciinema struct {
	Timestamp time.Time
	FilePath  string

	SessionId string
	Writer    *os.File
	InChan    chan string
	buf       []string
	HasWidth  bool
}

func NewAsciinema(sessionId string, pty gossh.Pty) (asc *Asciinema, err error) {
	asc = &Asciinema{
		Timestamp: time.Now(),
		InChan:    make(chan string, 20480),
		SessionId: sessionId,
	}
	if config.SSHConfig.RecordFilePath == "" {
		asc.FilePath, _ = os.Getwd()
	}
	asc.FilePath = filepath.Join(asc.FilePath, sessionId+".cast")

	castFile, err := os.Create(asc.FilePath)
	if err != nil {
		return
	}
	asc.Writer = castFile

	head := map[string]any{
		"version":   2,
		"width":     pty.Window.Width,
		"height":    pty.Window.Height,
		"timestamp": asc.Timestamp.Unix(),
		"env": map[string]any{
			"SHELL": "/bin/bash",
			"TERM":  "xterm-256color",
		},
	}
	s, _ := json.Marshal(head)
	if pty.Window.Width == 0 {
		asc.buf = append(asc.buf, string(s))
	} else {
		asc.HasWidth = true
		_, err = castFile.Write(s)
		if err != nil {
			return asc, err
		}
		_, err = castFile.WriteString("\r\n")
		if err != nil {
			return asc, err
		}
	}
	return asc, err
}

func (a *Asciinema) Write(data []byte) error {
	s := make([]any, 3)
	s[0] = (float64(time.Now().UnixMicro() - a.Timestamp.UnixMicro())) / 1000 / 1000
	s[1] = "o"
	s[2] = string(data)
	res, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if !a.HasWidth {
		a.buf = append(a.buf, string(res))
	} else {
		_, err = a.Writer.Write(append(res, []byte("\r\n")...))
	}
	return err
}

func (a *Asciinema) RemoteWrite(rec string) {
	a.InChan <- rec
	for v := range a.InChan {
		err := api.AddReplay(a.SessionId, map[string]string{
			"session_id": a.SessionId,
			"body":       v,
		})
		if err != nil {
			logger.L.Error(err.Error())
			break
		}
	}
}

func (a *Asciinema) Close() {
	a.Writer.Close()
	err := api.AddReplayFile(a.SessionId, a.FilePath)
	if err != nil {
		logger.L.Error(err.Error())
	}
	err = os.Remove(a.FilePath)
	if err != nil {
		logger.L.Warn(err.Error(), zap.String("module", "asciinema"))
	}
}

func (a *Asciinema) Resize(height, width int) error {
	if !a.HasWidth {
		a.ReWriteZeroWidth(height, width)
	}
	s := make([]any, 3)
	s[0] = (float64(time.Now().UnixMicro() - a.Timestamp.UnixMicro())) / 1000 / 1000
	s[1] = "r"
	s[2] = fmt.Sprintf("%dx%d", width, height)

	res, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = a.Writer.Write(append(res, []byte("\r\n")...))
	return err
}

func (a *Asciinema) ReWriteZeroWidth(height, width int) {
	defer func() {
		a.HasWidth = true
	}()
	if len(a.buf) > 0 {
		head := map[string]any{}
		er := json.Unmarshal([]byte(a.buf[0]), &head)
		if er == nil {
			head["width"] = width
			head["height"] = height
			s, er := json.Marshal(head)
			if er == nil {
				a.buf[0] = string(s)
			}
		}
	}

	_, _ = a.Writer.WriteString(strings.Join(a.buf, "\r\n"))
	_, _ = a.Writer.WriteString("\r\n")
	return
}
