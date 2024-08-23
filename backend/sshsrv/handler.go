package sshsrv

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gliderlabs/ssh"
	"github.com/veops/oneterm/logger"
)

func handler(s ssh.Session) {
	pty, winCh, isPty := s.Pty()
	if !isPty {
		logger.L().Error("not a pty request")
		return
	}

	// p := tea.NewProgram(initialModel(), tea.WithInput(io.NopCloser(s)), tea.WithOutput(NopWriteCloser(s)))
	p := tea.NewProgram(initialView(s, pty, winCh), tea.WithInput(s), tea.WithOutput(s))

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
}

func banner(ctx ssh.Context) string {
	return "\n--------------------------oneterm-----------------------\n"
}
