// Package handler
/**
Copyright (c) The Authors.
* @Author: feng.xiang
* @Date: 2024/1/24 15:20
* @Desc:
*/
package handler

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.uber.org/zap"

	myi18n "github.com/veops/oneterm/pkg/i18n"
	"github.com/veops/oneterm/pkg/logger"
)

type TermModel struct {
	table           table.Model
	query           string
	cookie          string
	Object          *InteractiveHandler
	SearchTime      time.Time
	Rows            []table.Row
	Step            int
	PreView         int
	lang            string
	lastOutputLines int
	in              *ProxyReader
	out             *ProxyWriter
	hostExit        bool
	hasMsg          chan struct{}
}

const (
	Tip int = iota
	ChooseHost
	ChooseAccount
	HostInteractive
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func (m *TermModel) Init() tea.Cmd { return nil }

func (m *TermModel) addStep(v int) {
	switch m.Step {
	case Tip, ChooseHost:
		m.Step += v
	case ChooseAccount:
		m.Step -= v
	default:
		m.Step = Tip
	}
}

func (m *TermModel) printfToSSH(out string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.Object.Session.Write([]byte(out + "\n"))
		if err != nil {
			return err
		}
		return nil
	}
}

func (m *TermModel) CurrentState() int {
	return Tip
}

func (m *TermModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd tea.Cmd
		err error
	)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			sr := m.table.SelectedRow()
			if len(sr) == 0 {
				break
			}
			s := m.query
			m.query = ""
			switch s {
			case "/?":
				m.Step = Tip
				return m, nil
			default:
				switch len(sr) {
				case 0:
					return m, nil
				default:
					m.ClearDataSource()
					m.Step, err = m.Object.Proxy(sr[1], 0)
					if err != nil {
						logger.L.Warn(err.Error())
					}
					m.ResetSource()

					m.convertRows()
					m.updateTable()

					if len(m.Object.MessageChan) > 0 {
						out := <-m.Object.MessageChan
						return m, tea.Batch(tea.ClearScreen, tea.Printf(out))
					}

					if len(m.Object.AccountsForSelect) > 0 {
						m.Object.AccountsForSelect = nil
					}

					return m, tea.ClearScreen
				}
			}
		case "backspace":
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
			}
			m.updateTable()
		default:
			if msg.Type == tea.KeyRunes && len(msg.String()) >= 1 {
				m.query += msg.String()
			}
			switch m.query {
			case "/?":
				m.Step = Tip
				m.query = ""
				return m, nil
			case "/s":
				m.Step = Tip
				m.query = ""
				m.Object.SwitchLanguage("")
				return m, nil
			case "/q":
				return m, tea.Quit
			case "/*":
				m.Step = Tip
				m.query = ""
			}
			if m.Step == Tip {
				m.Step = ChooseHost
			}
			m.updateTable()
		}

	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

//func (m *TermModel) Welcome() []table.Row {
//	rows := []table.Row{
//		{"/?", "help"},
//		{"/s", "swith"},
//	}
//}

func (m *TermModel) View() string {
	defer func() {
		m.in.hasMsg.Store(false)
	}()

	var s string
	switch m.Step {
	case Tip:
		//m.table.SetColumns([]table.Column{
		//{Title: "Tip", Width: 80},
		//})
		m.PreView = Tip
		s += m.Object.Message(myi18n.MsgSshWelcome, map[string]any{"User": m.Object.Session.User()})
		s = "\x1b[2K\x1b[G" + s + "\n: " + m.query
	case ChooseHost:
		m.table.SetColumns([]table.Column{
			{Title: "No.", Width: 5},
			{Title: "Name", Width: 20},
			{Title: "Ip", Width: 18},
		})

		if m.PreView == Tip {
			s = "\x1b[2K\x1b[G"
		}
		s += baseStyle.Render(m.table.View() + "\n: " + m.query)
	case ChooseAccount:
		m.table.SetColumns([]table.Column{
			{Title: "No.", Width: 5},
			{Title: "Name", Width: 20},
			{Title: "Account", Width: 18},
		})
		s += baseStyle.Render(m.table.View() + "\n: " + m.query)
	}
	return s
	//m.lastOutputLines = strings.Count(s, "\n") + 1
	//moveToBottomRight := "\033[999;999H"
	//moveToBottomRight = ""
	//return moveToBottomRight + s
}

func (m *TermModel) clearLastOutput() string {
	return fmt.Sprintf("\033[%dA\033[J", m.lastOutputLines)
}

func (m *TermModel) updateTable() {
	if m.query == "" {
		m.table.SetRows(m.Rows)
		return
	}

	var filteredRows []table.Row
	for _, row := range m.Rows {
		if rowContainsQuery(row, m.query) {
			filteredRows = append(filteredRows, row)
		}
	}
	m.table.SetRows(filteredRows)
}

func rowContainsQuery(row table.Row, query string) bool {
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), strings.ToLower(query)) {
			return true
		}
	}
	return false
}

func InitAndRunTerm(obj *InteractiveHandler) *tea.Program {
	columns := []table.Column{
		{Title: "No.", Width: 5},
		{Title: "Name", Width: 20},
		{Title: "Ip", Width: 18},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	tm := &TermModel{
		table:  t,
		Object: obj,
		in:     &ProxyReader{r: obj.Session},
		out:    &ProxyWriter{w: obj.Session},
		hasMsg: make(chan struct{}),
	}

	tm.updateRows()
	return tea.NewProgram(tm, tea.WithInput(tm.in), tea.WithOutput(tm.out))
}

func (m *TermModel) updateRows() {
	assets, err := m.Object.AcquireAssets("", 0)
	if err != nil {
		logger.L.Warn(err.Error(), zap.String("module", "term"))
		return
	}
	var rows []table.Row
	for index, v1 := range assets {
		rows = append(rows, []string{strconv.Itoa(index), v1.Name, v1.Ip})
	}

	m.Object.Locker.Lock()
	m.Object.Assets = assets
	m.Object.Locker.Unlock()
	m.SearchTime = time.Now()
	m.Rows = rows
}

func (m *TermModel) convertRows() {
	switch m.Step {
	case ChooseAccount:
		var rows []table.Row
		for index, v := range m.Object.AccountsForSelect {
			rows = append(rows, table.Row{strconv.Itoa(index), v.Name, v.Account})
		}
		m.Rows = rows
	default:
		if time.Since(m.SearchTime) < time.Minute {
			var rows []table.Row
			for index, v1 := range m.Object.Assets {
				rows = append(rows, []string{strconv.Itoa(index), v1.Name, v1.Ip})
			}
			m.Rows = rows
			return
		}
		m.updateRows()
	}
}

type nilWriter struct{}

func (nw nilWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (m *TermModel) ClearDataSource() {
	m.in.SetReader(nil)
	m.out.SetWriter(nil)
}

func (m *TermModel) ResetSource() {
	m.in.SetReader(m.Object.Session)
	m.out.SetWriter(m.Object.Session)
}

type ProxyWriter struct {
	lock sync.RWMutex
	w    io.Writer
}

func (pw *ProxyWriter) Write(p []byte) (n int, err error) {
	pw.lock.RLock()
	defer pw.lock.RUnlock()
	for pw.w == nil {
		time.Sleep(time.Millisecond * 50)
	}
	return pw.w.Write(p)
}

func (pw *ProxyWriter) SetWriter(w io.Writer) {
	pw.lock.Lock()
	defer pw.lock.Unlock()
	pw.w = w
}

type ProxyReader struct {
	lock   sync.RWMutex
	r      io.Reader
	hasMsg atomic.Bool
}

func (pr *ProxyReader) Read(p []byte) (n int, err error) {
	pr.lock.RLock()
	defer pr.lock.RUnlock()
	for pr.r == nil || pr.hasMsg.Load() {
		time.Sleep(time.Millisecond * 100)
	}

	n, err = pr.r.Read(p)
	pr.hasMsg.Store(true)
	return
}

func (pr *ProxyReader) SetReader(r io.Reader) {
	pr.r = r
}
