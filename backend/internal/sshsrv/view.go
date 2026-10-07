package sshsrv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/cancelreader"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/internal/acl"
	myConnector "github.com/veops/oneterm/internal/connector"
	"github.com/veops/oneterm/internal/connector/protocols"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/sshsrv/assetlist"
	"github.com/veops/oneterm/internal/sshsrv/colors"
	"github.com/veops/oneterm/pkg/cache"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	prompt     = "> "
	hisCmdsFmt = "hiscmds-%d"
)

var (
	errStyle     = colors.ErrorStyle
	hintStyle    = colors.HintStyle
	warningStyle = colors.WarningStyle
	hiddenBorder = lipgloss.HiddenBorder()

	p2p = map[string]int{
		"ssh":        22,
		"redis":      6379,
		"mysql":      3306,
		"mongodb":    27017,
		"postgresql": 5432,
		"telnet":     23,
	}
)

func init() {
	hiddenBorder.Left = "  "
}

type errMsg error

type connectionEndedMsg struct{ err error }
type outputPrintedMsg struct{}

type keymap struct{}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up/down"), key.WithHelp("↑/↓", "navigate suggestions")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "auto-complete")),
		key.NewBinding(key.WithKeys("f5"), key.WithHelp("F5", "refresh")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

type viewMode int

const (
	modeCLI viewMode = iota
	modeTable
)

type view struct {
	Ctx           *gin.Context
	Sess          terminalSession
	currentUser   *acl.Session
	textinput     textinput.Model
	assetTable    assetlist.Model
	spinner       spinner.Model
	cmds          []string
	cmdsIdx       int
	combines      map[string][3]int
	connections   []assetlist.Asset
	directTargets map[string][3]int
	suggestions   []connectionSuggestion
	matchInput    string
	matchReady    bool
	matches       []string
	matchCount    int
	matchPrefix   string
	historySaved  int
	connecting    bool
	help          help.Model
	height        int
	cliHeight     int
	width         int
	cursorText    string
	cursorPos     int
	cursorWidth   int
	cursorX       int
	keys          keymap
	r             io.ReadCloser
	w             io.WriteCloser
	gctx          context.Context
	mode          viewMode
	suggestionIdx int    // Track current suggestion selection
	selectedSugg  string // Store the selected suggestion text
}

type directCandidate struct {
	target    [3]int
	ambiguous bool
}

type connectionSuggestion struct {
	command string
	lower   string
}

func addDirectCandidate(candidates map[string]directCandidate, alias string, target [3]int) {
	if alias == "" || strings.ContainsAny(alias, " \t\r\n") {
		return
	}
	if candidate, ok := candidates[alias]; ok {
		if candidate.target != target {
			candidate.ambiguous = true
			candidates[alias] = candidate
		}
		return
	}
	candidates[alias] = directCandidate{target: target}
}

func addAccountDirectCandidates(candidates map[string]directCandidate, account *model.Account, target [3]int) {
	addDirectCandidate(candidates, account.Name, target)
	addDirectCandidate(candidates, account.Account, target)
}

func uniqueDirectTargets(candidates map[string]directCandidate) map[string][3]int {
	targets := make(map[string][3]int, len(candidates))
	for alias, candidate := range candidates {
		if !candidate.ambiguous {
			targets[alias] = candidate.target
		}
	}
	return targets
}

func (m *view) setConnectionSuggestions() {
	m.suggestions = make([]connectionSuggestion, 0, len(m.combines)+len(m.directTargets))
	for command := range m.combines {
		m.suggestions = append(m.suggestions, connectionSuggestion{command, strings.ToLower(command)})
	}
	for alias := range m.directTargets {
		command := "ssh " + alias
		m.suggestions = append(m.suggestions, connectionSuggestion{command, strings.ToLower(command)})
	}
	sort.Slice(m.suggestions, func(i, j int) bool {
		if m.suggestions[i].lower == m.suggestions[j].lower {
			return m.suggestions[i].command < m.suggestions[j].command
		}
		return m.suggestions[i].lower < m.suggestions[j].lower
	})
	unique := m.suggestions[:0]
	for _, suggestion := range m.suggestions {
		if len(unique) == 0 || unique[len(unique)-1].command != suggestion.command {
			unique = append(unique, suggestion)
		}
	}
	m.suggestions = unique
	m.matchReady = false
	m.textinput.ShowSuggestions = true
	m.textinput.SetSuggestions(nil)
}

func initialView(ctx *gin.Context, sess terminalSession, r io.ReadCloser, w io.WriteCloser, gctx context.Context) *view {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	ti := textinput.New()
	ti.Placeholder = "Type 'help' or start with 'ssh user@host'..."
	ti.Focus()
	ti.Prompt = prompt
	ti.ShowSuggestions = true
	ti.SetVirtualCursor(false)
	styles := ti.Styles()
	styles.Focused.Prompt = colors.PrimaryStyle
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(colors.TextSecondary)
	styles.Cursor.Color = colors.PrimaryColor9
	ti.SetStyles(styles)
	ti.KeyMap.AcceptSuggestion.SetEnabled(false)

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = colors.PrimaryStyle

	v := view{
		Ctx:           ctx,
		Sess:          sess,
		currentUser:   currentUser,
		textinput:     ti,
		spinner:       s,
		cmds:          []string{},
		help:          help.New(),
		r:             r,
		w:             w,
		gctx:          gctx,
		mode:          modeCLI,
		suggestionIdx: 0,
	}
	v.refresh()
	v.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#9a9a9a"))
	v.help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colors.TextSecondary)
	v.help.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colors.TextDisabled)

	return &v
}

func (m *view) Init() tea.Cmd {
	return textinput.Blink
}

func welcomeMessage() string {
	return fmt.Sprintf("%s\n  %s\n\n  %s\n\n", banner(),
		colors.AccentStyle.Render("→ Welcome to OneTerm! Start typing or use 'ls' to browse assets"),
		colors.HintStyle.Render("Examples: ssh admin@server1, mysql db@prod, redis cache@redis"))
}

func (m *view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = max(1, size.Width)
		m.height = max(1, size.Height)
		m.textinput.SetWidth(max(1, size.Width-lipgloss.Width(prompt)-1))
		m.help.SetWidth(max(1, size.Width-2))
		if m.suggestionIdx >= m.suggestionLimit() {
			m.suggestionIdx = 0
			m.selectedSugg = ""
		}
	}
	var (
		hisCmd     tea.Cmd
		tiCmd      tea.Cmd
		tableCmd   tea.Cmd
		spinnerCmd tea.Cmd
	)

	// Update spinner if connecting
	if m.connecting {
		m.spinner, spinnerCmd = m.spinner.Update(msg)
	}

	// Handle table mode
	if m.mode == modeTable {
		switch msg := msg.(type) {
		case assetlist.ConnectMsg:
			// Handle connection from table
			m.mode = modeCLI
			cmd := msg.Asset.Command
			return m, m.handleConnectionCommand(cmd)
		case assetlist.BackMsg:
			return m, m.focusCLI()
		}
		m.assetTable, tableCmd = m.assetTable.Update(msg)
		return m, tableCmd
	}

	// Handle CLI mode
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			// Clear current input like in terminal, don't quit
			m.textinput.Reset()
			m.textinput.ShowSuggestions = true
			m.textinput.SetSuggestions(nil)
			m.suggestionIdx = 0
			m.selectedSugg = ""
			return m, m.textinput.Focus()
		case "esc":
			m.selectedSugg = ""
			m.suggestionIdx = 0
			return m, nil
		case "enter":
			// Use selected suggestion if one is selected, otherwise use typed value
			cmd := m.textinput.Value()
			if m.selectedSugg != "" {
				cmd = m.selectedSugg
			}
			m.textinput.Reset()
			m.textinput.ShowSuggestions = true
			m.textinput.SetSuggestions(nil)
			m.selectedSugg = ""
			m.suggestionIdx = 0
			if cmd == "" {
				return m, m.textinput.Focus()
			}
			hisCmd = m.printf("%s%s", prompt, cmd)
			m.cmds = append(m.cmds, cmd)
			ln := len(m.cmds)
			if ln > 100 {
				m.cmds = m.cmds[ln-100 : ln]
				m.historySaved = max(0, m.historySaved-(ln-100))
			}
			m.cmdsIdx = len(m.cmds)

			switch {
			case cmd == "exit" || cmd == "quit" || cmd == `\q`:
				return m, tea.Sequence(m.printf("Goodbye."), tea.Quit)
			case cmd == "help" || cmd == `\h` || cmd == `\?`:
				return m, tea.Batch(m.textinput.Focus(), tea.Sequence(hisCmd, m.printf("%s", m.helpText())))
			case cmd == "clear" || cmd == `\c`:
				return m, tea.ClearScreen
			case cmd == "list" || cmd == "ls" || cmd == "table":
				pty, _, _ := m.Sess.Pty()
				// Ensure we have reasonable default dimensions if pty size is not available
				width := pty.Window.Width
				height := pty.Window.Height
				if width <= 0 {
					width = 80 // Standard terminal width
				}
				if height <= 0 {
					height = 24 // Standard terminal height
				}
				m.assetTable = assetlist.NewConnections(m.connections, width, height)
				m.mode = modeTable
				// Send a window size message to ensure consistent initial state
				sizeMsg := tea.WindowSizeMsg{Width: width, Height: height}
				m.assetTable, _ = m.assetTable.Update(sizeMsg)
				return m, nil
			case cmd == "recent" || cmd == "r" || cmd == `\r`:
				// Show recent sessions in table mode
				pty, _, _ := m.Sess.Pty()
				width := pty.Window.Width
				height := pty.Window.Height
				if width <= 0 {
					width = 80
				}
				if height <= 0 {
					height = 24
				}

				// Get recent sessions (filtered at database level)
				sessions, err := m.getRecentSessions()
				if err != nil {
					return m, tea.Sequence(
						hisCmd,
						m.printf("\n  %s %v\n\n", errStyle.Render("Failed to fetch recent sessions:"), err),
					)
				}

				if len(sessions) == 0 {
					return m, tea.Sequence(
						hisCmd,
						m.printf("\n  %s\n\n", hintStyle.Render("No recent sessions found")),
					)
				}

				// Create recent sessions table
				m.assetTable = assetlist.NewRecentSessions(sessions, m.combines, width, height, m.connections)
				m.mode = modeTable
				sizeMsg := tea.WindowSizeMsg{Width: width, Height: height}
				m.assetTable, _ = m.assetTable.Update(sizeMsg)
				return m, nil
			}

			// Try to handle as connection command
			if connectionCmd := m.handleConnectionCommand(cmd); connectionCmd != nil {
				return m, tea.Sequence(
					hisCmd,
					connectionCmd,
				)
			} else {
				var suggestion string
				if strings.Contains(cmd, "@") {
					suggestion = "\n  Try: ssh " + cmd + " (if connecting via SSH)"
				} else {
					suggestion = "\n  Available commands: ssh, mysql, redis, mongodb, postgresql, telnet, help, list, exit"
				}
				return m, tea.Sequence(
					hisCmd,
					m.printf("  %s %s%s\n\n",
						errStyle.Render("Unknown command:"),
						cmd,
						hintStyle.Render(suggestion),
					),
				)
			}
		case "up":
			// If we have suggestions and input is not empty, navigate suggestions
			input := m.textinput.Value()
			if len(input) > 0 {
				suggestions := m.getFilteredSuggestions(input)
				if len(suggestions) > 0 && m.suggestionLimit() > 0 {
					if m.selectedSugg == "" {
						m.suggestionIdx = 0
						m.selectedSugg = suggestions[0]
					} else if m.suggestionIdx > 0 {
						m.suggestionIdx--
						if m.suggestionIdx < len(suggestions) {
							m.selectedSugg = suggestions[m.suggestionIdx]
						}
					}
					return m, nil
				}
			}
			// Otherwise navigate command history
			ln := len(m.cmds)
			if ln <= 0 {
				return m, nil
			}
			m.cmdsIdx = max(0, m.cmdsIdx-1)
			m.textinput.SetValue(m.cmds[m.cmdsIdx])
			m.suggestionIdx = 0
			m.selectedSugg = ""
		case "down":
			// If we have suggestions and input is not empty, navigate suggestions
			input := m.textinput.Value()
			if len(input) > 0 {
				suggestions := m.getFilteredSuggestions(input)
				if len(suggestions) > 0 && m.suggestionLimit() > 0 {
					limit := min(m.suggestionLimit(), len(suggestions))
					if m.selectedSugg == "" {
						m.suggestionIdx = 0
						m.selectedSugg = suggestions[0]
					} else if m.suggestionIdx < limit-1 {
						m.suggestionIdx++
						if m.suggestionIdx < len(suggestions) {
							m.selectedSugg = suggestions[m.suggestionIdx]
						}
					}
					return m, nil
				}
			}
			// Otherwise navigate command history
			ln := len(m.cmds)
			m.cmdsIdx = min(m.cmdsIdx+1, ln)
			if m.cmdsIdx == ln {
				m.textinput.SetValue("")
			} else {
				m.textinput.SetValue(m.cmds[m.cmdsIdx])
			}
			m.suggestionIdx = 0
			m.selectedSugg = ""
		case "f5":
			m.selectedSugg = ""
			m.suggestionIdx = 0
			m.refresh()
		case "tab":
			// Auto-complete with common prefix or selected suggestion
			input := m.textinput.Value()
			if input == "" {
				return m, nil
			}

			suggestions := m.getFilteredSuggestions(input)
			if len(suggestions) == 0 {
				return m, nil
			}

			if m.selectedSugg != "" {
				m.textinput.SetValue(m.selectedSugg)
				m.textinput.CursorEnd()
				m.selectedSugg = ""
				m.suggestionIdx = 0
			} else if m.matchCount == 1 {
				// Single match - complete fully
				m.textinput.SetValue(suggestions[0])
				m.textinput.CursorEnd() // Move cursor to end
				m.selectedSugg = ""
				m.suggestionIdx = 0
			} else {
				// Multiple matches - complete to common prefix
				commonPrefix := m.matchPrefix
				if len(commonPrefix) > len(input) {
					m.textinput.SetValue(commonPrefix)
					m.textinput.CursorEnd() // Move cursor to end
					m.selectedSugg = ""
					m.suggestionIdx = 0
				}
			}
		}
	case errMsg:
		if msg != nil {
			return m, tea.Batch(m.textinput.Focus(), m.printf("  [ERROR] %s\n", errStyle.Render(msg.Error())))
		}
	case connectionEndedMsg:
		m.connecting = false
		m.textinput.SetSuggestions(nil)
		if msg.err != nil {
			return m, tea.Batch(m.textinput.Focus(), m.printf("  [ERROR] %s\n", errStyle.Render(msg.err.Error())))
		}
		return m, m.textinput.Focus()
	}

	value := m.textinput.Value()
	m.textinput.ShowSuggestions = true
	m.textinput, tiCmd = m.textinput.Update(msg)
	if m.textinput.Value() != value {
		m.suggestionIdx = 0
		m.selectedSugg = ""
	}
	m.getFilteredSuggestions(m.textinput.Value())
	if m.matchPrefix != "" {
		prefix, input := []rune(m.matchPrefix), []rune(m.textinput.Value())
		if len(prefix) > len(input) {
			m.textinput.SetSuggestions([]string{string(input) + string(prefix[len(input):])})
		} else {
			m.textinput.SetSuggestions(nil)
		}
	} else {
		m.textinput.SetSuggestions(nil)
	}

	return m, tea.Batch(hisCmd, tiCmd, spinnerCmd)
}

func (m *view) filterMessage(_ tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(outputPrintedMsg); ok {
		if m.width <= 0 || m.height <= 0 {
			return nil
		}
		return tea.WindowSizeMsg{Width: m.width, Height: m.height}
	}
	return msg
}

func (m *view) focusCLI() tea.Cmd {
	m.mode = modeCLI
	m.selectedSugg = ""
	m.suggestionIdx = 0
	return m.textinput.Focus()
}

func (m *view) View() tea.View {
	input := ""
	if m.mode == modeCLI && !m.connecting {
		input = m.textinput.View()
	}
	v := tea.NewView(m.render(input))
	v.AltScreen = m.mode == modeTable
	if m.mode == modeCLI && !m.connecting {
		v.Content = strings.TrimRight(v.Content, " \n")
		m.cliHeight = max(m.cliHeight, lipgloss.Height(v.Content))
		if m.height > 0 {
			m.cliHeight = min(m.cliHeight, max(1, m.height-1))
		}
		v.Content = lipgloss.NewStyle().Height(m.cliHeight).MaxHeight(m.cliHeight).Render(v.Content)
		v.Cursor = m.inputCursor(input)
	}
	return v
}

func (m *view) printf(format string, args ...any) tea.Cmd {
	width := m.width
	if width <= 0 {
		width = 80
	}
	// Insert one physical row at a time, preserving styles across wraps.
	lines := uv.NewStyledString(ansi.Hardwrap(fmt.Sprintf(format, args...), width, true)).Lines(ansi.WcWidth)
	cmds := make([]tea.Cmd, 0, len(lines)+1)
	for _, line := range lines {
		text := line.String()
		if text == "" {
			text = " "
		}
		cmds = append(cmds, tea.Printf("%s", text))
	}
	// Unmanaged output moves the cursor even when the view is unchanged.
	cmds = append(cmds, func() tea.Msg { return outputPrintedMsg{} })
	return tea.Sequence(cmds...)
}

func (m *view) inputCursor(text string) *tea.Cursor {
	cur := m.textinput.Cursor()
	if cur == nil {
		return nil
	}
	pos, width := m.textinput.Position(), m.textinput.Width()
	if text == m.cursorText && pos == m.cursorPos && width == m.cursorWidth {
		cur.X = m.cursorX
		return cur
	}
	// Use the editor's visible cursor to account for Unicode and scrolling.
	input := m.textinput
	input.SetVirtualCursor(true)
	styles := input.Styles()
	styles.Cursor.Blink = false
	input.SetStyles(styles)
	lines := uv.NewStyledString(input.View()).Lines(ansi.WcWidth)
	for x, cell := range lines[0] {
		if cell.Style.Attrs&uv.AttrReverse != 0 {
			cur.X = x
			break
		}
	}
	if m.width > 0 {
		cur.X = min(cur.X, m.width-1)
	}
	m.cursorText, m.cursorPos, m.cursorWidth, m.cursorX = text, pos, width, cur.X
	return cur
}

func (m *view) suggestionLimit() int {
	if m.height <= 0 {
		return 8
	}
	return min(8, max(0, m.height-7))
}

func (m *view) render(input string) string {
	if m.connecting {
		return m.renderConnectingStatus()
	}

	if m.mode == modeTable {
		// The table already handles clearing and formatting internally
		return m.assetTable.View()
	}

	suggestionView := m.smartSuggestionView()

	return fmt.Sprintf(
		"%s\n  %s\n%s%s",
		input,
		m.help.View(m.keys),
		suggestionView,
		m.assetOverview(),
	) + "\n\n"
}

func (m *view) smartSuggestionView() string {
	// Get all suggestions and filter them ourselves for better matching
	input := m.textinput.Value()
	if input == "" {
		return ""
	}

	// Use our consistent filtered suggestions function
	matches := m.getFilteredSuggestions(input)
	ln := m.matchCount
	if ln <= 0 {
		return ""
	}

	// Clean and validate matches before displaying
	cleanMatches := make([]string, 0, len(matches))
	for _, match := range matches {
		match = strings.TrimSpace(match)
		// Only filter out truly empty matches
		if match != "" {
			cleanMatches = append(cleanMatches, match)
		}
	}

	if len(cleanMatches) == 0 {
		return ""
	}

	limit := min(m.suggestionLimit(), len(cleanMatches))
	if limit == 0 {
		return ""
	}
	displaySuggestions := cleanMatches[:limit]

	var result strings.Builder
	suggestTitle := colors.SubtitleStyle
	result.WriteString("\n  " + suggestTitle.Render("Suggestions:") + "\n")

	// Render each suggestion
	for i, suggestion := range displaySuggestions {
		// Render with appropriate style
		if i == m.suggestionIdx && m.selectedSugg != "" {
			selectedStyle := colors.HighlightStyle
			result.WriteString(fmt.Sprintf("  → %s\n", selectedStyle.Render(suggestion)))
		} else {
			// Use a lighter color for non-selected suggestions on dark background
			normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
			result.WriteString(fmt.Sprintf("    %s\n", normalStyle.Render(suggestion)))
		}
	}

	// Show count if there are more suggestions
	if ln > limit {
		moreStyle := lipgloss.NewStyle().
			Foreground(colors.TextSecondary).
			Italic(true)
		result.WriteString("  " + moreStyle.Render(fmt.Sprintf("... +%d more. Keep typing to filter.", ln-limit)) + "\n")
	}

	return result.String()
}

// Keep one row so Exec resumes at the target's final cursor.
func (m *view) renderConnectingStatus() string {
	return fmt.Sprintf("  %s Connecting...", m.spinner.View())
}

func (m *view) helpText() string {
	return fmt.Sprintf(`%s

%s
  • ssh user@host        - Connect via SSH
  • mysql user@host      - Connect to MySQL database  
  • redis user@host      - Connect to Redis server
  • mongodb user@host    - Connect to MongoDB database
  • postgresql user@host - Connect to PostgreSQL database
  • telnet user@host     - Connect via Telnet
  • list/ls/table        - Show assets in interactive table
  • recent or r or \r    - Show recent sessions with last login time
  • help or \h or \?     - Show this help message
  • clear or \c          - Clear screen
  • exit/quit or \q      - Exit OneTerm

%s
  • Use ↑/↓ arrows to browse command history
  • Press Tab to autocomplete connection names
  • Press Ctrl+C to clear current input
  • Press F5 to refresh asset list

`,
		colors.TitleStyle.Render("OneTerm Help"),
		hintStyle.Render("Available Commands:"),
		hintStyle.Render("Keyboard Shortcuts:"),
	)
}

func (m *view) handleConnectionCommand(cmd string) tea.Cmd {
	target, p, ok := m.resolveConnectionTarget(cmd)
	if !ok {
		return nil
	}

	// Setup connection parameters
	pty, _, _ := m.Sess.Pty()

	// Create a copy of the context first to avoid modifying the original
	newCtx := m.Ctx.Copy()

	// Ensure Request and URL are properly initialized
	if newCtx.Request == nil {
		newCtx.Request = &http.Request{
			RemoteAddr: m.Sess.RemoteAddr().String(),
			URL:        &url.URL{},
		}
	}
	if newCtx.Request.URL == nil {
		newCtx.Request.URL = &url.URL{}
	}

	newCtx.Request = newCtx.Request.Clone(m.gctx)
	newCtx.Request.URL.RawQuery = fmt.Sprintf("w=%d&h=%d", pty.Window.Width, pty.Window.Height)
	newCtx.Params = nil
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "account_id", Value: cast.ToString(target[0])})
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(target[1])})
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "protocol", Value: fmt.Sprintf("%s:%d", p, target[2])})
	newCtx.Set("sessionType", model.SESSIONTYPE_CLIENT)
	m.connecting = true

	return tea.Sequence(
		m.printf("\n  %s\n", colors.AccentStyle.Render(fmt.Sprintf("Connecting to %s", cmd))),
		// Start spinner and connection in background
		m.spinner.Tick,
		tea.Exec(&connector{Ctx: newCtx, Sess: m.Sess, gctx: m.gctx}, func(err error) tea.Msg { return connectionEndedMsg{err: err} }),
	)
}

func (m *view) resolveConnectionTarget(cmd string) (target [3]int, protocol string, ok bool) {
	if target, ok = m.combines[cmd]; ok {
		protocol, ok = lo.Find(lo.Keys(p2p), func(item string) bool { return strings.HasPrefix(cmd, item) })
		return target, protocol, ok
	}

	parts := strings.Fields(cmd)
	if len(parts) != 2 || parts[0] != "ssh" || m.directTargets == nil {
		return [3]int{}, "", false
	}
	target, ok = m.directTargets[parts[1]]
	return target, "ssh", ok
}

func (m *view) assetOverview() string {
	if len(m.textinput.Value()) > 0 {
		return "" // Hide overview when user is typing
	}

	if len(m.combines) == 0 {
		return warningStyle.Render("\n  No accessible assets found. Check your permissions.")
	}

	// Provide a better tip with modern styling
	textStyle := lipgloss.NewStyle().
		Foreground(colors.TextSecondary)

	cmdStyle := lipgloss.NewStyle().
		Foreground(colors.PrimaryColor9).
		Bold(true)

	arrowStyle := lipgloss.NewStyle().
		Foreground(colors.PrimaryColor2)

	// Build the tip text with each part styled correctly
	parts := []string{
		arrowStyle.Render("→"),
		textStyle.Render("Type"),
		cmdStyle.Render("'ls'"),
		textStyle.Render("for interactive mode,"),
		cmdStyle.Render("'recent'"),
		textStyle.Render("for recent sessions, or start typing to connect"),
	}

	fullTip := strings.Join(parts, " ")
	return lipgloss.NewStyle().PaddingTop(1).Render(fullTip)
}

func (m *view) refresh() {
	eg := &errgroup.Group{}
	eg.Go(func() (err error) {
		assets, err := repository.GetAllFromCacheDb(m.gctx, model.DefaultAsset)
		if err != nil {
			return
		}
		accounts, err := repository.GetAllFromCacheDb(m.gctx, model.DefaultAccount)
		if err != nil {
			return
		}
		if !acl.IsAdmin(m.currentUser) {
			var assetIds []int

			// Use V2 authorization system for asset filtering
			authV2Service := service.NewAuthorizationV2Service()
			if _, assetIds, _, err = authV2Service.GetAuthorizationScopeByACL(m.Ctx); err != nil {
				return
			}
			assetSet := make(map[int]struct{}, len(assetIds))
			for _, id := range assetIds {
				assetSet[id] = struct{}{}
			}
			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { _, ok := assetSet[a.Id]; return ok })
			accountSet := make(map[int]struct{})
			for _, asset := range assets {
				for id := range asset.Authorization {
					accountSet[id] = struct{}{}
				}
			}
			accounts = lo.Filter(accounts, func(a *model.Account, _ int) bool { _, ok := accountSet[a.Id]; return ok })
		}

		accountMap := lo.SliceToMap(accounts, func(a *model.Account) (int, *model.Account) { return a.Id, a })

		m.combines = make(map[string][3]int)
		m.connections = nil
		directCandidates := make(map[string]directCandidate)
		for _, asset := range assets {
			for accountId, authData := range asset.Authorization {
				account, ok := accountMap[accountId]
				if !ok {
					continue
				}

				// Check if this account has connect permission
				if authData.Permissions == nil || !authData.Permissions.Connect {
					continue
				}

				for _, p := range asset.Protocols {
					ss := strings.Split(p, ":")
					if len(ss) != 2 {
						continue
					}
					protocol := ss[0]
					defaultPort, ok := p2p[protocol]
					if !ok {
						continue
					}
					k := fmt.Sprintf("%s %s@%s", protocol, account.Name, asset.Name)
					port := cast.ToInt(ss[1])
					if port <= 0 || port > 65535 {
						continue
					}
					// Ensure we're not creating empty or malformed keys
					if k != "" && len(k) > 3 {
						m.combines[lo.Ternary(port == defaultPort, k, fmt.Sprintf("%s:%s", k, ss[1]))] = [3]int{account.Id, asset.Id, port}
						command := lo.Ternary(port == defaultPort, k, fmt.Sprintf("%s:%s", k, ss[1]))
						m.connections = append(m.connections, assetlist.Asset{Protocol: protocol, Command: command, User: account.Name, Host: asset.Name, Port: ss[1], Info: [3]int{account.Id, asset.Id, port}})
					}
					if protocol == "ssh" {
						target := [3]int{account.Id, asset.Id, port}
						addAccountDirectCandidates(directCandidates, account, target)
					}
				}
			}
		}
		m.directTargets = uniqueDirectTargets(directCandidates)
		m.setConnectionSuggestions()

		return
	})

	eg.Go(func() error {
		var err error
		if len(m.cmds) != 0 {
			return err
		}
		m.cmds, err = cache.RC.LRange(m.Ctx, fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid()), -100, -1).Result()
		m.cmdsIdx = len(m.cmds)
		m.historySaved = len(m.cmds)
		return err
	})

	if err := eg.Wait(); err != nil {
		m.combines, m.directTargets = nil, nil
		m.connections = nil
		m.setConnectionSuggestions()
		logger.L().Error("refresh failed", zap.Error(err))
		return
	}

}

func (m *view) getRecentSessions() ([]*model.Session, error) {
	sessionRepo := repository.NewSessionRepository()
	return sessionRepo.GetRecentSessionsByUser(m.gctx, m.currentUser.GetUid(), 20)
}

func (m *view) RecordHisCmd() {
	if m.historySaved >= len(m.cmds) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	k := fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid())
	pipe := cache.RC.TxPipeline()
	pipe.RPush(ctx, k, m.cmds[m.historySaved:])
	pipe.LTrim(ctx, k, -100, -1)
	pipe.Expire(ctx, k, time.Hour*24*30)
	if _, err := pipe.Exec(ctx); err != nil {
		logger.L().Debug("save terminal history failed", zap.Error(err))
		return
	}
	m.historySaved = len(m.cmds)
}

// getFilteredSuggestions returns suggestions that match the input
func (m *view) getFilteredSuggestions(input string) []string {
	if m.matchReady && m.matchInput == input {
		return m.matches
	}
	m.matchInput, m.matchReady = input, true
	m.matches, m.matchCount, m.matchPrefix = nil, 0, ""
	if input == "" {
		return nil
	}
	inputLower := strings.ToLower(input)
	start := sort.Search(len(m.suggestions), func(i int) bool { return m.suggestions[i].lower >= inputLower })
	end := start + sort.Search(len(m.suggestions)-start, func(i int) bool { return !strings.HasPrefix(m.suggestions[start+i].lower, inputLower) })
	for start < end && m.suggestions[start].lower == inputLower {
		start++
	}
	m.matchCount = end - start
	if m.matchCount == 0 {
		return nil
	}
	m.matchPrefix = m.findCommonPrefix([]string{m.suggestions[start].command, m.suggestions[end-1].command})
	m.matches = make([]string, 0, min(m.matchCount, 20))
	for _, suggestion := range m.suggestions[start:min(end, start+20)] {
		m.matches = append(m.matches, suggestion.command)
	}
	return m.matches
}

// findCommonPrefix finds the longest common prefix among suggestions
func (m *view) findCommonPrefix(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	if len(suggestions) == 1 {
		return suggestions[0]
	}

	// Start with the first suggestion
	prefix := []rune(suggestions[0])

	// Compare with each other suggestion
	for _, s := range suggestions[1:] {
		runes := []rune(s)
		// Find common prefix between current prefix and this suggestion
		i := 0
		minLen := min(len(prefix), len(runes))
		for i < minLen && strings.EqualFold(string(prefix[i]), string(runes[i])) {
			i++
		}
		prefix = prefix[:i]

		if len(prefix) == 0 {
			return ""
		}
	}

	return string(prefix)
}

type connector struct {
	Ctx    *gin.Context
	Sess   terminalSession
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	gctx   context.Context
}

func (conn *connector) SetStdin(r io.Reader) {
	conn.stdin = r
}

func (conn *connector) SetStdout(w io.Writer) {
	if output, ok := w.(*terminalOutput); ok {
		w = output.writer
	}
	conn.stdout = w
}

func (conn *connector) SetStderr(w io.Writer) {
	if output, ok := w.(*terminalOutput); ok {
		w = output.writer
	}
	conn.stderr = w
}

func startConnectorInputRelay(input io.Reader, output *io.PipeWriter) (func(), error) {
	duplicate, err := duplicateInputReader(input)
	if err != nil {
		return nil, err
	}
	reader, err := cancelreader.NewReader(duplicate)
	if err != nil {
		_ = duplicate.Close()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer output.Close()
		_, _ = io.Copy(output, reader)
	}()
	return func() {
		if !reader.Cancel() {
			cancelDuplicateInputReader(duplicate)
		}
		_ = output.Close()
		<-done
		_ = reader.Close()
		_ = duplicate.Close()
	}, nil
}

func (conn *connector) Run() error {
	gsess, err := myConnector.DoConnect(conn.Ctx, nil)
	if err != nil {
		return err
	}

	r, w := io.Pipe()
	stopInput, err := startConnectorInputRelay(conn.stdin, w)
	if err != nil {
		_ = r.Close()
		_ = w.Close()
		gsess.Once.Do(func() { close(gsess.Chans.AwayChan) })
		gsess.G.Wait()
		return err
	}

	gsess.CliRw = &session.CliRW{
		Reader: bufio.NewReader(r),
		Writer: conn.stdout,
	}

	_, ch, ok := conn.Sess.Pty()
	if !ok {
		ch = make(<-chan ssh.Window)
	}
	gsess.G.Go(func() (err error) {
		defer r.Close()
		defer w.Close()
		for {
			select {
			case <-gsess.Chans.AwayChan:
				return
			case <-conn.gctx.Done():
				gsess.Once.Do(func() { close(gsess.Chans.AwayChan) })
				return
			case <-gsess.Gctx.Done():
				return
			case window, open := <-ch:
				if !open {
					ch = nil
					continue
				}
				gsess.Chans.Resize(window)
			}
		}
	})
	err = myConnector.HandleTerm(gsess, conn.Ctx)
	_ = r.Close()
	stopInput()

	if err != nil {
		// Check if this is the normal termination sentinel error
		if err.Error() == "session closed normally" {
			logger.L().Debug("sshsrv session ended normally", zap.String("sessionId", gsess.SessionId))
		} else {
			logger.L().Debug("sshsrv run stopped", zap.String("sessionId", gsess.SessionId), zap.Error(err))
		}
	}

	conn.stdout.Write([]byte("\r\n\r\n"))

	if errors.Is(err, io.EOF) || errors.Is(err, protocols.ErrSessionClosed) {
		return nil
	}
	return err
}
