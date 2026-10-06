package sshsrv

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gin-gonic/gin"
	"github.com/gliderlabs/ssh"
	"github.com/muesli/cancelreader"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/api/controller"
	myConnector "github.com/veops/oneterm/internal/connector"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/repository"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/sshsrv/assetlist"
	"github.com/veops/oneterm/internal/sshsrv/colors"
	"github.com/veops/oneterm/internal/sshsrv/icons"
	"github.com/veops/oneterm/internal/sshsrv/textinput"
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

type keymap struct{}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up/down"), key.WithHelp("↑/↓", "navigate suggestions")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "auto-complete")),
		key.NewBinding(key.WithKeys("f5"), key.WithHelp("F5", "refresh")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit")),
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
	Sess          ssh.Session
	currentUser   *acl.Session
	textinput     textinput.Model
	assetTable    assetlist.Model
	spinner       spinner.Model
	cmds          []string
	cmdsIdx       int
	combines      map[string][3]int
	directTargets map[string][3]int
	suggestions   []connectionSuggestion
	connecting    bool
	help          help.Model
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
	sort.Slice(m.suggestions, func(i, j int) bool { return m.suggestions[i].lower < m.suggestions[j].lower })
}

func initialView(ctx *gin.Context, sess ssh.Session, r io.ReadCloser, w io.WriteCloser, gctx context.Context) *view {
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	ti := textinput.New()
	ti.Placeholder = "Type 'help' or start with 'ssh user@host'..."
	ti.Focus()
	ti.Prompt = prompt
	ti.ShowSuggestions = true
	ti.PromptStyle = colors.PrimaryStyle
	ti.Cursor.Style = colors.AccentStyle
	// Disable Tab for AcceptSuggestion to handle it ourselves
	ti.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("ctrl+x")) // Use a key that won't be pressed

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

	return &v
}

func (m *view) Init() tea.Cmd {
	welcomeStyle := colors.AccentStyle
	exampleStyle := colors.HintStyle

	return tea.Batch(
		tea.Println(banner()),
		tea.Printf("\n  %s\n\n", welcomeStyle.Render("→ Welcome to OneTerm! Start typing or use 'ls' to browse assets")),
		tea.Printf("  %s\n", exampleStyle.Render("Examples: ssh admin@server1, mysql db@prod, redis cache@redis")),
	)
}

func (m *view) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		// Let table handle the message first
		m.assetTable, tableCmd = m.assetTable.Update(msg)

		// Check for special messages after table has processed them
		switch msg := msg.(type) {
		case assetlist.ConnectMsg:
			// Handle connection from table
			m.mode = modeCLI
			cmd := msg.Asset.Command
			return m, m.handleConnectionCommand(cmd)
		case assetlist.BackMsg:
			// Exit table mode when Back is triggered
			m.mode = modeCLI
			return m, tea.Printf("\r%s", prompt)
		case tea.KeyMsg:
			// Only exit on Esc/q if filter is NOT active
			if !m.assetTable.IsFilterActive() && (msg.Type == tea.KeyEsc || msg.String() == "q") {
				// Exit table mode
				m.mode = modeCLI
				return m, tea.Printf("\r%s", prompt)
			}
		}

		return m, tableCmd
	}

	// Handle CLI mode
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			// Clear current input like in terminal, don't quit
			m.textinput.Reset()
			m.suggestionIdx = 0
			m.selectedSugg = ""
			return m, tea.Printf("\n%s", prompt)
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			// Use selected suggestion if one is selected, otherwise use typed value
			cmd := m.textinput.Value()
			if m.selectedSugg != "" {
				cmd = m.selectedSugg
			}
			m.textinput.Reset()
			m.selectedSugg = ""
			m.suggestionIdx = 0
			if cmd == "" {
				return m, tea.Batch(tea.Printf(prompt))
			}
			hisCmd = tea.Printf("🚀 %s", cmd)
			m.cmds = append(m.cmds, cmd)
			ln := len(m.cmds)
			if ln > 100 {
				m.cmds = m.cmds[ln-100 : ln]
			}
			m.cmdsIdx = len(m.cmds)

			switch {
			case cmd == "exit" || cmd == "quit" || cmd == `\q`:
				return m, tea.Sequence(tea.Printf("👋 Goodbye!"), tea.Quit)
			case cmd == "help" || cmd == `\h` || cmd == `\?`:
				return m, tea.Sequence(hisCmd, tea.Printf(m.helpText()), tea.Printf("%s", prompt))
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
				m.assetTable = assetlist.New(m.combines, width, height)
				m.mode = modeTable
				// Send a window size message to ensure consistent initial state
				sizeMsg := tea.WindowSizeMsg{Width: width, Height: height}
				m.assetTable, _ = m.assetTable.Update(sizeMsg)
				return m, tea.ClearScreen
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
						tea.Printf("\n  %s Failed to fetch recent sessions: %v\n\n", errStyle.Render("⚠️"), err),
						tea.Printf("%s", prompt),
					)
				}

				if len(sessions) == 0 {
					return m, tea.Sequence(
						hisCmd,
						tea.Printf("\n  %s\n\n", hintStyle.Render("📋 No recent sessions found")),
						tea.Printf("%s", prompt),
					)
				}

				// Create recent sessions table
				m.assetTable = assetlist.NewRecentSessions(sessions, m.combines, width, height)
				m.mode = modeTable
				sizeMsg := tea.WindowSizeMsg{Width: width, Height: height}
				m.assetTable, _ = m.assetTable.Update(sizeMsg)
				return m, tea.ClearScreen
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
					suggestion = "\n💪 Try: ssh " + cmd + " (if connecting via SSH)"
				} else {
					suggestion = "\n💪 Available commands: ssh, mysql, redis, mongodb, postgresql, telnet, help, list, exit"
				}
				return m, tea.Sequence(
					hisCmd,
					tea.Printf("  %s %s%s\n\n",
						errStyle.Render("⚠️ Unknown command:"),
						cmd,
						hintStyle.Render(suggestion),
					),
					tea.Printf("%s", prompt),
				)
			}
		case tea.KeyUp:
			// If we have suggestions and input is not empty, navigate suggestions
			input := m.textinput.Value()
			if len(input) > 0 {
				suggestions := m.getFilteredSuggestions(input)
				if len(suggestions) > 0 {
					if m.suggestionIdx > 0 {
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
		case tea.KeyDown:
			// If we have suggestions and input is not empty, navigate suggestions
			input := m.textinput.Value()
			if len(input) > 0 {
				suggestions := m.getFilteredSuggestions(input)
				if len(suggestions) > 0 {
					limit := min(8, len(suggestions))
					if m.suggestionIdx < limit-1 {
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
			m.cmdsIdx++
			if m.cmdsIdx >= ln {
				m.cmdsIdx = ln - 1
				m.textinput.SetValue("")
			} else {
				m.textinput.SetValue(m.cmds[m.cmdsIdx])
			}
			m.suggestionIdx = 0
			m.selectedSugg = ""
		case tea.KeyF5:
			m.refresh()
		case tea.KeyTab:
			// Auto-complete with common prefix or selected suggestion
			input := m.textinput.Value()
			if input == "" {
				return m, nil
			}

			suggestions := m.getFilteredSuggestions(input)
			if len(suggestions) == 0 {
				return m, nil
			}

			if len(suggestions) == 1 {
				// Single match - complete fully
				m.textinput.SetValue(suggestions[0])
				m.textinput.CursorEnd() // Move cursor to end
				m.selectedSugg = ""
				m.suggestionIdx = 0
			} else {
				// Multiple matches - complete to common prefix
				commonPrefix := m.findCommonPrefix(suggestions)
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
			return m, tea.Printf("  [ERROR] %s\n\n%s", errStyle.Render(msg.Error()), prompt)
		}
	}

	// Reset suggestion index and selected when typing
	if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyRunes {
		m.suggestionIdx = 0
		m.selectedSugg = ""
	}

	m.textinput, tiCmd = m.textinput.Update(msg)

	return m, tea.Batch(hisCmd, tiCmd, spinnerCmd)
}

func (m *view) View() string {
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
		m.textinput.View(),
		m.help.View(m.keys),
		suggestionView,
		m.assetOverview(),
	) + "\n\n"
}

func (m *view) smartSuggestionView() string {
	// Get all suggestions and filter them ourselves for better matching
	input := strings.ToLower(m.textinput.Value())
	if input == "" {
		return ""
	}

	// Use our consistent filtered suggestions function
	matches := m.getFilteredSuggestions(input)
	ln := len(matches)
	if ln <= 0 {
		return ""
	}

	if ln > 20 {
		countStyle := lipgloss.NewStyle().
			Foreground(colors.TextSecondary).
			Italic(true)
		return "\n  " + countStyle.Render(fmt.Sprintf("%d matches found. Keep typing to filter...", ln)) + "\n"
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

	limit := min(8, len(cleanMatches))
	displaySuggestions := cleanMatches[:limit]

	// Ensure suggestion index is within bounds
	if m.suggestionIdx >= limit {
		m.suggestionIdx = limit - 1
	}

	var result strings.Builder
	suggestTitle := colors.SubtitleStyle
	result.WriteString("\n  " + suggestTitle.Render("Suggestions:") + "\n")

	// Render each suggestion
	for i, suggestion := range displaySuggestions {
		// Get protocol for icon
		parts := strings.Fields(suggestion)
		protocol := "unknown"
		if len(parts) > 0 {
			protocol = parts[0]
		}
		icon := icons.GetStyledProtocolIcon(protocol)

		// Render with appropriate style
		if i == m.suggestionIdx {
			selectedStyle := colors.HighlightStyle
			result.WriteString(fmt.Sprintf("  → %s %s\n", icon, selectedStyle.Render(suggestion)))
		} else {
			// Use a lighter color for non-selected suggestions on dark background
			normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
			result.WriteString(fmt.Sprintf("    %s %s\n", icon, normalStyle.Render(suggestion)))
		}
	}

	// Show count if there are more suggestions
	if len(cleanMatches) > limit {
		moreStyle := lipgloss.NewStyle().
			Foreground(colors.TextSecondary).
			Italic(true)
		result.WriteString("  " + moreStyle.Render(fmt.Sprintf("... +%d more", len(cleanMatches)-limit)) + "\n")
	}

	return result.String()
}

// renderConnectingStatus displays animated connecting status
func (m *view) renderConnectingStatus() string {
	return fmt.Sprintf("\n  %s Connecting...\n\n", m.spinner.View())
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
		colors.TitleStyle.Render("🌟 OneTerm Help"),
		hintStyle.Render("📝 Available Commands:"),
		hintStyle.Render("⌨️ Keyboard Shortcuts:"),
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

	newCtx.Request.URL.RawQuery = fmt.Sprintf("w=%d&h=%d", pty.Window.Width, pty.Window.Height)
	newCtx.Params = nil
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "account_id", Value: cast.ToString(target[0])})
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "asset_id", Value: cast.ToString(target[1])})
	newCtx.Params = append(newCtx.Params, gin.Param{Key: "protocol", Value: fmt.Sprintf("%s:%d", p, target[2])})
	newCtx.Set("sessionType", model.SESSIONTYPE_CLIENT)
	m.connecting = true

	return tea.Sequence(
		tea.Printf("\n  %s %s\n",
			colors.PrimaryStyle.Render("⚡"),
			colors.AccentStyle.Render(fmt.Sprintf("Initiating secure connection to %s", cmd))),
		// Start spinner and connection in background
		m.spinner.Tick,
		tea.Exec(&connector{Ctx: newCtx, Sess: m.Sess, gctx: m.gctx}, func(err error) tea.Msg {
			m.connecting = false
			if err != nil {
				return errMsg(fmt.Errorf("%s Connection failed: %v",
					colors.ErrorStyle.Render("✗"), err))
			}
			// Return a success message to update the UI
			return tea.Printf("\r\n%s Connection established successfully.\r\n%s",
				colors.PrimaryStyle.Render("✓"), prompt)
		}),
		tea.Printf("%s", prompt),
		func() tea.Msg {
			m.textinput.ClearMatched()
			return nil
		},
		m.magicn,
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
		return warningStyle.Render("\n  ⚠ No accessible assets found. Check your permissions.")
	}

	// Group assets by protocol for better organization
	protocolGroups := make(map[string][]string)
	for cmd := range m.combines {
		parts := strings.Split(cmd, " ")
		if len(parts) > 0 {
			protocol := parts[0]
			protocolGroups[protocol] = append(protocolGroups[protocol], cmd)
		}
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
			var assetIds, accountIds []int

			// Use V2 authorization system for asset filtering
			authV2Service := service.NewAuthorizationV2Service()
			if _, assetIds, _, err = authV2Service.GetAuthorizationScopeByACL(m.Ctx); err != nil {
				return
			}
			assets = lo.Filter(assets, func(a *model.Asset, _ int) bool { return lo.Contains(assetIds, a.Id) })

			if accountIds, err = controller.GetAccountIdsByAuthorization(m.Ctx); err != nil {
				return
			}
			accounts = lo.Filter(accounts, func(a *model.Account, _ int) bool { return lo.Contains(accountIds, a.Id) })
		}

		accountMap := lo.SliceToMap(accounts, func(a *model.Account) (int, *model.Account) { return a.Id, a })

		m.combines = make(map[string][3]int)
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
					// Ensure we're not creating empty or malformed keys
					if k != "" && len(k) > 3 {
						m.combines[lo.Ternary(port == defaultPort, k, fmt.Sprintf("%s:%s", k, ss[1]))] = [3]int{account.Id, asset.Id, port}
					}
					if protocol == "ssh" {
						target := [3]int{account.Id, asset.Id, port}
						addAccountDirectCandidates(directCandidates, account, target)
					}
				}
			}
		}
		m.directTargets = uniqueDirectTargets(directCandidates)
		m.textinput.SetSuggestions(lo.Keys(m.combines))
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
		return err
	})

	if err := eg.Wait(); err != nil {
		logger.L().Error("refresh failed", zap.Error(err))
		return
	}

}

func (m *view) magicn() tea.Msg {
	m.w.Write([]byte("\n"))
	return nil
}

func (m *view) getRecentSessions() ([]*model.Session, error) {
	sessionRepo := repository.NewSessionRepository()
	return sessionRepo.GetRecentSessionsByUser(m.gctx, m.currentUser.GetUid(), 20)
}

func (m *view) RecordHisCmd() {
	k := fmt.Sprintf(hisCmdsFmt, m.currentUser.GetUid())
	cache.RC.RPush(m.Ctx, k, m.cmds)
	cache.RC.LTrim(m.Ctx, k, -100, -1)
	cache.RC.Expire(m.Ctx, k, time.Hour*24*30)
}

// getFilteredSuggestions returns suggestions that match the input
func (m *view) getFilteredSuggestions(input string) []string {
	if input == "" {
		return nil
	}

	inputLower := strings.ToLower(input)
	var matches []string
	start := sort.Search(len(m.suggestions), func(i int) bool { return m.suggestions[i].lower >= inputLower })
	for _, suggestion := range m.suggestions[start:] {
		if !strings.HasPrefix(suggestion.lower, inputLower) {
			break
		}
		if len(suggestion.command) > len(input) {
			matches = append(matches, suggestion.command)
		}
	}

	// Sort matches for consistent ordering
	sort.Strings(matches)

	// Remove any duplicates (shouldn't happen but just in case)
	if len(matches) > 1 {
		unique := make([]string, 0, len(matches))
		prev := ""
		for _, m := range matches {
			if m != prev {
				unique = append(unique, m)
				prev = m
			}
		}
		matches = unique
	}

	return matches
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
	prefix := suggestions[0]

	// Compare with each other suggestion
	for _, s := range suggestions[1:] {
		// Find common prefix between current prefix and this suggestion
		i := 0
		minLen := min(len(prefix), len(s))
		for i < minLen && strings.EqualFold(string(prefix[i:i+1]), string(s[i:i+1])) {
			i++
		}
		prefix = prefix[:i]

		if len(prefix) == 0 {
			return ""
		}
	}

	return prefix
}

type connector struct {
	Ctx    *gin.Context
	Sess   ssh.Session
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	gctx   context.Context
}

func (conn *connector) SetStdin(r io.Reader) {
	conn.stdin = r
}

func (conn *connector) SetStdout(w io.Writer) {
	conn.stdout = w
}

func (conn *connector) SetStderr(w io.Writer) {
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
			case w := <-ch:
				// Non-blocking send to WindowChan
				// Some protocols (like telnet) don't handle window changes
				select {
				case gsess.Chans.WindowChan <- w:
				default:
					// If no one is listening, just ignore
				}
			}
		}
	})
	myConnector.HandleTerm(gsess, nil)
	_ = r.Close()
	stopInput()

	if err = gsess.G.Wait(); err != nil {
		// Check if this is the normal termination sentinel error
		if err.Error() == "session closed normally" {
			logger.L().Debug("sshsrv session ended normally", zap.String("sessionId", gsess.SessionId))
		} else {
			logger.L().Debug("sshsrv run stopped", zap.String("sessionId", gsess.SessionId), zap.Error(err))
		}
	}

	conn.stdout.Write([]byte("\n\n"))

	return nil
}
