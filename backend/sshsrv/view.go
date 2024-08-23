package sshsrv

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gliderlabs/ssh"
	"github.com/samber/lo"
)

const (
	hotPink  = lipgloss.Color("#FF06B7")
	darkGray = lipgloss.Color("#767676")
)

var (
	inputStyle = lipgloss.NewStyle().Foreground(hotPink)
	hintStyle  = lipgloss.NewStyle().Foreground(darkGray)
)

type view struct {
	cmds        []string
	id          string
	textinput   textinput.Model
	senderStyle lipgloss.Style
	err         error
	Sess        ssh.Session
	cmdsIdx     int
}

func initialView(s ssh.Session, pty ssh.Pty, winCh chan<- ssh.Window) view {
	//todo combine hint
	ti := textinput.New()
	ti.Placeholder = "ssh"
	ti.Focus()
	ti.Width = 30
	ti.Prompt = "> "
	ti.ShowSuggestions = true
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	return view{
		textinput:   ti,
		cmds:        []string{},
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:         nil,
		Sess:        s,
	}
}

func (m view) Init() tea.Cmd {
	return textarea.Blink
}

func (m view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		hisCmd tea.Cmd
		tiCmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.textinput.Value() == "" {
				return m, tea.Batch(tea.Printf("> "))
			}
			hisCmd = tea.Printf("> %s", m.textinput.Value())
			m.cmds = append(m.cmds, m.textinput.Value())
			m.textinput.Reset()
			m.cmdsIdx = len(m.cmds) - 1
			return m, tea.Exec(&connector{}, func(err error) tea.Msg { return err })
		case tea.KeyUp:
			ln := len(m.cmds)
			if ln <= 0 || m.cmdsIdx < 0 {
				return m, nil
			}
			m.cmdsIdx--
			m.textinput.SetValue(m.cmds[m.cmdsIdx])
		case tea.KeyDown:
			ln := len(m.cmds)
			if ln <= 0 || m.cmdsIdx == ln-1 {
				return m, nil
			}
			m.cmdsIdx++
			m.textinput.SetValue(m.cmds[m.cmdsIdx])
		}

		// We handle errors just like any other message
	case error:
		m.err = msg
		return m, nil
	}
	m.textinput, tiCmd = m.textinput.Update(msg)

	return m, tea.Batch(hisCmd, tiCmd)
}

func (m view) View() string {
	return fmt.Sprintf(
		"%s\n%s",
		m.textinput.View(),
		hintStyle.Render(m.possible()),
	) + "\n\n"
}

func (m *view) possible() string {
	cur := m.textinput
	res := lo.Filter(cur.AvailableSuggestions(), func(s string, _ int) bool {
		return strings.HasPrefix(strings.ToLower(s), strings.ToLower(cur.Value()))
	})
	return lo.Ternary(len(res) == 0, "", fmt.Sprintf("%s", res))
}

type connector struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (s *connector) SetStdin(r io.Reader) {
	s.stdin = r
}

func (s *connector) SetStdout(w io.Writer) {
	s.stdout = w
}

func (s *connector) SetStderr(w io.Writer) {
	s.stderr = w
}

func (s *connector) Run() error {


	// sess.Stdout = s.stdout
	// sess.Stderr = s.stdout
	// sess.Stdin = s.stdin


}
