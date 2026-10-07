package session

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

const outputWindow = 256 * 1024

type OutputFlow struct {
	mu          sync.Mutex
	sent, acked uint64
	notify      chan struct{}
}

func (s *Session) StartOutputFlow() error {
	if s.Gctx == nil || s.Session == nil || !s.BinaryOutput || s.IsGuacd() {
		return fmt.Errorf("output flow control is unavailable")
	}
	if s.OutputFlow != nil {
		return nil
	}
	s.OutputFlow = &OutputFlow{notify: make(chan struct{}, 1)}
	return s.WriteWebsocket(websocket.TextMessage, []byte("0flow"))
}

func (s *Session) HandleOutputAck(p []byte) (bool, error) {
	if len(p) == 0 || p[0] != 'a' || s.IsGuacd() {
		return false, nil
	}
	if s.OutputFlow == nil {
		return true, nil
	}
	if len(p) < 2 || len(p) > 21 {
		return true, fmt.Errorf("invalid output acknowledgement")
	}
	count, err := strconv.ParseUint(string(p[1:]), 10, 64)
	if err != nil {
		return true, fmt.Errorf("invalid output acknowledgement")
	}
	f := s.OutputFlow
	f.mu.Lock()
	if count > f.sent {
		f.mu.Unlock()
		return true, fmt.Errorf("output acknowledgement exceeds sent bytes")
	}
	if count <= f.acked {
		f.mu.Unlock()
		return true, nil
	}
	f.acked = count
	f.mu.Unlock()
	select {
	case f.notify <- struct{}{}:
	default:
	}
	return true, nil
}

func (f *OutputFlow) reserve(ctx, sessionCtx context.Context, count int) error {
	if count < 0 || count > outputWindow {
		return fmt.Errorf("invalid output size")
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := sessionCtx.Err(); err != nil {
			return err
		}
		f.mu.Lock()
		if uint64(count) <= outputWindow-(f.sent-f.acked) {
			if f.sent > math.MaxUint64-uint64(count) {
				f.mu.Unlock()
				return fmt.Errorf("output counter overflow")
			}
			f.sent += uint64(count)
			f.mu.Unlock()
			return nil
		}
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-sessionCtx.Done():
			return sessionCtx.Err()
		case <-f.notify:
		}
	}
}
