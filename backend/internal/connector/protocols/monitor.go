package protocols

import (
	"sync"

	"github.com/gorilla/websocket"
	gsession "github.com/veops/oneterm/internal/session"
)

type Monitor struct {
	sess  *gsession.Session
	queue chan []byte
	done  chan struct{}
	once  sync.Once
}

func NewMonitor(ws *websocket.Conn, binary bool) *Monitor {
	m := &Monitor{sess: &gsession.Session{Ws: ws, BinaryOutput: binary}, queue: make(chan []byte, 32), done: make(chan struct{})}
	go func() {
		defer m.Close()
		for {
			select {
			case <-m.done:
				return
			case data := <-m.queue:
				if err := m.sess.WriteTerminal(data); err != nil {
					return
				}
			}
		}
	}()
	return m
}

func (m *Monitor) Close() { m.once.Do(func() { close(m.done); m.sess.Ws.Close() }) }

func (m *Monitor) Write(data []byte) {
	for len(data) > 0 {
		n := min(len(data), 32768)
		select {
		case <-m.done:
			return
		case m.queue <- data[:n]:
			data = data[n:]
		default:
			m.Close()
			return
		}
	}
}
