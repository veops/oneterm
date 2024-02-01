package handler

import (
	"fmt"

	gossh "github.com/gliderlabs/ssh"

	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/proto/ssh/config"
)

func RegisterMonitorSession(sessionId string, sess gossh.Session) {
	_, ok := config.TotalHostSession.Load(sessionId)
	if !ok {
		return
	}

	config.TotalMonitors.LoadOrStore(sessionId, sess)

	if _, ok := config.TotalMonitors.Load(sessionId); !ok {
		config.TotalMonitors.Store(sessionId, sess)
	}

	<-sess.Context().Done()
	DeleteMonitorSession(sessionId)
}

func DeleteMonitorSession(sessionId string) {
	config.TotalMonitors.Delete(sessionId)
}

func getMonitorSession(sessionId string) gossh.Session {
	if v, ok := config.TotalMonitors.Load(sessionId); ok {
		return v.(gossh.Session)
	}
	return nil
}

func Monitor(sessionId string, p []byte) {
	if s := getMonitorSession(sessionId); s != nil {
		_, err := s.Write(p)
		if err != nil {
			logger.L.Error(fmt.Sprintf("moninor session %s failed: %s", sessionId, err.Error()))
			return
		}
	}
}
