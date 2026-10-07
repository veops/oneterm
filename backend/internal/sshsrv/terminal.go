package sshsrv

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"golang.org/x/sync/errgroup"
)

type terminalOutput struct {
	writer io.Writer
	mu     sync.Mutex
	buffer []byte
	last   byte

	terminal    *terminalState
	alternate   bool
	redraw      []byte
	clear       []byte
	clearHeight int
	erases      []screenErase
}

type screenErase struct {
	inputStart, inputEnd   int
	outputStart, outputEnd int
}

func (w *terminalOutput) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Tea expects a PTY to map line feeds; remote transports send raw bytes.
	source := p
	mapLines := bytes.IndexByte(p, '\n') >= 0
	if mapLines {
		w.buffer = w.buffer[:0]
		previous := w.last
		for _, b := range p {
			if b == '\n' && previous != '\r' {
				w.buffer = append(w.buffer, '\r')
			}
			w.buffer = append(w.buffer, b)
			previous = b
		}
		source = w.buffer
	}
	alternate := w.alternate
	output := w.rewriteScreen(source)
	written, err := w.writer.Write(output)
	if written == len(output) {
		if len(p) > 0 {
			w.last = p[len(p)-1]
		}
		return len(p), err
	}
	if err == nil {
		err = io.ErrShortWrite
	}
	// Map a partial transport write back to the original input.
	used := written
	for _, erase := range w.erases {
		if written < erase.outputStart {
			break
		}
		if written < erase.outputEnd {
			used = erase.inputStart
			break
		}
		used -= erase.outputEnd - erase.outputStart - (erase.inputEnd - erase.inputStart)
	}
	w.alternate = alternate
	w.rewriteScreen(source[:used])
	previous := w.last
	for n < len(p) && used > 0 {
		size := 1
		if mapLines && p[n] == '\n' && previous != '\r' {
			size++
		}
		if used < size {
			break
		}
		used -= size
		previous = p[n]
		n++
	}
	if n > 0 {
		w.last = p[n-1]
	}
	return
}

func (w *terminalOutput) rewriteScreen(p []byte) []byte {
	// Tea emits complete control sequences in its buffered renderer writes.
	const erase = ansi.CursorHomePosition + ansi.EraseEntireScreen
	w.erases = w.erases[:0]
	if w.terminal == nil || (!w.alternate && !bytes.Contains(p, []byte(ansi.SetModeAltScreenSaveCursor))) ||
		(w.alternate && !bytes.Contains(p, []byte(erase)) && !bytes.Contains(p, []byte(ansi.ResetModeAltScreenSaveCursor))) {
		return p
	}
	w.redraw = w.redraw[:0]
	offset := 0
	for position := 0; position < len(p); {
		index := bytes.IndexByte(p[position:], '\x1b')
		if index < 0 {
			break
		}
		position += index
		switch {
		case bytes.HasPrefix(p[position:], []byte(ansi.SetModeAltScreenSaveCursor)):
			w.alternate = true
			position += len(ansi.SetModeAltScreenSaveCursor)
		case bytes.HasPrefix(p[position:], []byte(ansi.ResetModeAltScreenSaveCursor)):
			w.alternate = false
			position += len(ansi.ResetModeAltScreenSaveCursor)
		case w.alternate && bytes.HasPrefix(p[position:], []byte(erase)):
			w.redraw = append(w.redraw, p[offset:position]...)
			start := len(w.redraw)
			w.redraw = append(w.redraw, w.clearScreen()...)
			w.erases = append(w.erases, screenErase{position, position + len(erase), start, len(w.redraw)})
			position += len(erase)
			offset = position
		default:
			position++
		}
	}
	if offset == 0 {
		return p
	}
	w.redraw = append(w.redraw, p[offset:]...)
	return w.redraw
}

func (w *terminalOutput) clearScreen() []byte {
	pty, _, _ := w.terminal.Pty()
	if w.clearHeight == pty.Window.Height {
		return w.clear
	}
	// Erase lines without adding the old alternate screen to scrollback.
	w.clear = w.clear[:0]
	for row := 1; row <= pty.Window.Height; row++ {
		w.clear = append(w.clear, "\x1b["...)
		w.clear = strconv.AppendInt(w.clear, int64(row), 10)
		w.clear = append(w.clear, 'H')
		w.clear = append(w.clear, ansi.EraseEntireLine...)
	}
	w.clear = append(w.clear, ansi.CursorHomePosition...)
	w.clearHeight = pty.Window.Height
	return w.clear
}

type terminalSession interface {
	Pty() (ssh.Pty, <-chan ssh.Window, bool)
	RemoteAddr() net.Addr
}

type terminalState struct {
	mu      sync.Mutex
	pty     ssh.Pty
	remote  net.Addr
	environ []string
	windows chan ssh.Window
	updates chan ssh.Window
}

func newTerminal(pty ssh.Pty, remote net.Addr, environ ...string) *terminalState {
	if pty.Term == "" {
		pty.Term = "xterm-256color"
	}
	if pty.Window.Width <= 0 {
		pty.Window.Width = 80
	}
	if pty.Window.Height <= 0 {
		pty.Window.Height = 24
	}
	environ = append(append([]string(nil), environ...), "TERM="+pty.Term)
	return &terminalState{pty: pty, remote: remote, environ: environ, windows: make(chan ssh.Window, 1), updates: make(chan ssh.Window, 1)}
}

func (t *terminalState) Pty() (ssh.Pty, <-chan ssh.Window, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pty, t.windows, true
}

func (t *terminalState) RemoteAddr() net.Addr { return t.remote }

func (t *terminalState) Resize(window ssh.Window) {
	if window.Width <= 0 || window.Height <= 0 || window.Width > 1000 || window.Height > 500 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pty.Window = window
	for _, ch := range []chan ssh.Window{t.windows, t.updates} {
		select {
		case <-ch:
		default:
		}
		ch <- window
	}
}

func runTerminal(ctx *gin.Context, terminal *terminalState, input io.ReadCloser, output io.Writer, windows <-chan ssh.Window) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer r.Close()
	defer w.Close()
	runCtx, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()
	ctx = ctx.Copy()
	ctx.Request = ctx.Request.Clone(runCtx)
	vw := initialView(ctx, terminal, r, w, runCtx)
	defer vw.RecordHisCmd()
	pty, _, _ := terminal.Pty()
	writer := &terminalOutput{writer: output, terminal: terminal}
	// Print before Tea starts so startup does not insert empty history.
	if _, err := io.WriteString(writer, welcomeMessage()); err != nil {
		return err
	}
	p := tea.NewProgram(vw, tea.WithContext(runCtx), tea.WithInput(r), tea.WithOutput(writer),
		tea.WithFilter(vw.filterMessage),
		tea.WithWindowSize(pty.Window.Width, pty.Window.Height), tea.WithEnvironment(terminal.environ),
		tea.WithColorProfile(colorprofile.Env(terminal.environ)))
	stop := context.AfterFunc(runCtx, func() { input.Close(); w.Close() })
	defer stop()
	terminal.Resize(pty.Window)
	eg, gctx := errgroup.WithContext(runCtx)
	eg.Go(func() error {
		defer cancel()
		_, err := io.Copy(w, input)
		return err
	})
	eg.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case size := <-terminal.updates:
				p.Send(tea.WindowSizeMsg{Width: size.Width, Height: size.Height})
			}
		}
	})
	eg.Go(func() error {
		for {
			select {
			case <-gctx.Done():
				return nil
			case size, open := <-windows:
				if !open {
					windows = nil
					continue
				}
				terminal.Resize(size)
			}
		}
	})
	eg.Go(func() error {
		defer cancel()
		defer input.Close()
		defer w.Close()
		_, err := p.Run()
		return err
	})
	return eg.Wait()
}
