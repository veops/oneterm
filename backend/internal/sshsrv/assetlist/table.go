package assetlist

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/sshsrv/icons"
)

// Styles for the table using primary color palette
var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7f97fa")) // Light primary for borders

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2f54eb")). // Primary color
			Bold(true).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")). // White text
			Background(lipgloss.Color("#3F75FF")). // Bright primary background
			Bold(true)
)

// KeyMap for table navigation
type TableKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Enter    key.Binding
	Back     key.Binding
	Filter   key.Binding
}

var DefaultTableKeyMap = TableKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "ctrl+u"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "ctrl+d"),
		key.WithHelp("pgdn", "page down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("home/g", "first"),
	),
	End: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("end/G", "last"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "connect"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "q"),
		key.WithHelp("esc/q", "back"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
}

// Asset represents a connection asset
type Asset struct {
	Protocol  string
	Command   string
	User      string
	Host      string
	Port      string
	Info      [3]int // [accountId, assetId, port]
	LastLogin *time.Time // Optional: last login time for recent sessions
}

// Model represents the asset list table model
type Model struct {
	table          table.Model
	assets         []Asset
	filteredAssets []Asset
	filter         string
	filterModel    FilterModel
	width          int
	height         int
	focused        bool
	keyMap         TableKeyMap
	showHelp       bool
	isRecent       bool // Whether this is a recent sessions table
}

// New creates a new asset list table
func New(assets map[string][3]int, width, height int) Model {
	// Convert assets map to structured list
	assetList := make([]Asset, 0, len(assets))
	for cmd, info := range assets {
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			protocol := parts[0]
			userHost := parts[1]

			// Parse user@host format
			var user, host string
			if idx := strings.Index(userHost, "@"); idx > 0 {
				user = userHost[:idx]
				host = userHost[idx+1:]
			} else {
				user = "unknown"
				host = userHost
			}

			// Extract port if present
			port := ""
			if len(parts) > 2 {
				// Format: "protocol user@host:port"
				if idx := strings.LastIndex(parts[len(parts)-1], ":"); idx > 0 {
					port = parts[len(parts)-1][idx+1:]
					host = strings.TrimSuffix(host, ":"+port)
				}
			}

			assetList = append(assetList, Asset{
				Protocol: protocol,
				Command:  cmd,
				User:     user,
				Host:     host,
				Port:     port,
				Info:     info,
			})
		}
	}

	// Assets are stored in the order they were found

	// Create table columns
	columns := []table.Column{
		{Title: "Protocol", Width: 12}, // Increased for emoji + text
		{Title: "User", Width: 15},
		{Title: "Host", Width: 25},
		{Title: "Port", Width: 8},
		{Title: "Command", Width: 40},
	}

	// Create table rows
	rows := make([]table.Row, len(assetList))
	for i, asset := range assetList {
		icon := icons.GetProtocolIcon(asset.Protocol)
		rows[i] = table.Row{
			fmt.Sprintf("%s %s", icon, strings.ToUpper(asset.Protocol)),
			asset.User,
			asset.Host,
			lo.Ternary(asset.Port != "", asset.Port, icons.GetDefaultPort(asset.Protocol)),
			asset.Command,
		}
	}

	// Table height includes header (1) + separator (1) + data rows
	viewportHeight := len(assetList) + 2
	if viewportHeight < 3 {
		viewportHeight = 3 // At least header + separator + 1 data row
	}

	// Calculate max available height (account for UI overhead)
	// Overhead: title(1) + header(2) + borders(2) + help(2) = 7 lines
	maxViewportHeight := height - 10
	if maxViewportHeight < 5 {
		maxViewportHeight = 5 // Minimum usable height
	}

	// Use actual row count for small sets, cap for large sets
	if viewportHeight > maxViewportHeight {
		viewportHeight = maxViewportHeight
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(viewportHeight),
	)

	// Style the table with primary colors
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#b1c9ff")). // Light accent
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#2f54eb")) // Primary color
	s.Selected = selectedStyle
	t.SetStyles(s)

	return Model{
		table:          t,
		assets:         assetList,
		filteredAssets: assetList,
		filterModel:    NewFilter(),
		width:          width,
		height:         height,
		focused:        false,
		keyMap:         DefaultTableKeyMap,
		showHelp:       true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle filter input first if active
	if m.filterModel.Active() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEscape:
				// Exit filter mode
				m.filterModel.SetActive(false)
				m.filter = ""
				m.updateFilter()
				return m, nil
			case tea.KeyEnter:
				// Connect to selected asset if available
				if m.table.SelectedRow() != nil && m.table.Cursor() < len(m.filteredAssets) {
					selected := m.filteredAssets[m.table.Cursor()]
					m.filterModel.SetActive(false)
					return m, connectCmd(selected)
				}
				// Otherwise just apply filter and exit filter mode
				m.filter = m.filterModel.Value()
				m.updateFilter()
				m.filterModel.SetActive(false)
				return m, nil
			case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
				// Allow navigation keys to pass through to table
				m.table, cmd = m.table.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}

			// For all other keys (including 'q'), update filter input
			prevFilter := m.filter
			m.filterModel, cmd = m.filterModel.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

			// Live filter update only if changed
			newFilter := m.filterModel.Value()
			if newFilter != prevFilter {
				m.filter = newFilter
				m.updateFilter()
				// Only reset cursor on first character or significant change
				if prevFilter == "" && newFilter != "" {
					// First character typed - reset to top
					m.table.GotoTop()
				}
			}
			return m, tea.Batch(cmds...)

		default:
			// Let filter handle other messages
			m.filterModel, cmd = m.filterModel.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:

		switch {
		case key.Matches(msg, m.keyMap.Enter):
			// Return selected asset for connection
			if m.table.SelectedRow() != nil && m.table.Cursor() < len(m.filteredAssets) {
				selected := m.filteredAssets[m.table.Cursor()]
				return m, connectCmd(selected)
			}

		case key.Matches(msg, m.keyMap.Back):
			return m, backCmd()

		case key.Matches(msg, m.keyMap.Filter):
			if m.filter != "" {
				// Clear existing filter
				m.filter = ""
				m.updateFilter()
				// Reset cursor to top after clearing filter
				m.table.GotoTop()
			} else {
				// Start filtering mode
				m.filterModel.SetActive(true)
				// Initialize filter to empty to prepare for input
				m.filter = ""
			}
			return m, nil

		case key.Matches(msg, m.keyMap.Up),
			key.Matches(msg, m.keyMap.Down),
			key.Matches(msg, m.keyMap.PageUp),
			key.Matches(msg, m.keyMap.PageDown),
			key.Matches(msg, m.keyMap.Home),
			key.Matches(msg, m.keyMap.End):
			// Let the table handle all navigation keys
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update table height based on new window size
		// Table height includes header + separator + data rows
		newHeight := len(m.filteredAssets) + 2
		if newHeight < 3 {
			newHeight = 3 // At least header + separator + 1 data row
		}

		// Calculate max available height
		maxHeight := msg.Height - 10
		if maxHeight < 5 {
			maxHeight = 5 // Minimum usable height
		}

		// Use actual row count for small sets, cap for large sets
		if newHeight > maxHeight {
			newHeight = maxHeight
		}
		m.table.SetHeight(newHeight)
	default:
		// Update table for other messages
		m.table, cmd = m.table.Update(msg)
	}

	return m, cmd
}

// View renders the table
func (m Model) View() string {
	resetCursor := "\r\033[0G"
	title := titleStyle.Render(lo.Ternary(m.isRecent, "📊  Recent Sessions", "🗂️  Available Assets"))

	// Filter indicator or input
	filterInfo := ""
	if m.filterModel.Active() {
		// Show filter input
		filterInfo = " " + m.filterModel.View()
	} else if m.filter != "" {
		// Show active filter
		filterInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8c8c8c")). // Secondary text
			Render(fmt.Sprintf(" (filtered: %s)", m.filter))
	}

	// Asset count
	countText := lo.Ternary(m.isRecent, "sessions", "assets")
	count := fmt.Sprintf("%d %s", len(m.filteredAssets), countText)
	if m.filter != "" && len(m.filteredAssets) != len(m.assets) {
		count = fmt.Sprintf("%d of %d %s", len(m.filteredAssets), len(m.assets), countText)
	}
	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8c8c8c")). // Secondary text
		Render(count)

	// Help text
	help := m.renderHelp()

	// Combine all elements
	header := lipgloss.JoinHorizontal(lipgloss.Left, title, filterInfo, " ", countStyle)

	// Use the table component's view directly
	tableView := m.table.View()

	// Apply base style with proper width
	// Don't apply height constraint, let the table manage its own viewport
	tableBox := baseStyle.
		Width(m.width - 2).
		Render(tableView)

	result := resetCursor + header + "\n" + tableBox + "\n" + help
	return result
}

// Helper functions

func (m *Model) updateFilter() {
	if m.filter == "" {
		m.filteredAssets = m.assets
	} else {
		filter := strings.ToLower(m.filter)
		m.filteredAssets = lo.Filter(m.assets, func(a Asset, _ int) bool {
			return strings.Contains(strings.ToLower(a.Command), filter) ||
				strings.Contains(strings.ToLower(a.Host), filter) ||
				strings.Contains(strings.ToLower(a.User), filter) ||
				strings.Contains(strings.ToLower(a.Protocol), filter)
		})
	}

	// Update table rows

	rows := make([]table.Row, len(m.filteredAssets))
	for i, asset := range m.filteredAssets {
		icon := icons.GetProtocolIcon(asset.Protocol)
		rows[i] = table.Row{
			fmt.Sprintf("%s %s", icon, strings.ToUpper(asset.Protocol)),
			asset.User,
			asset.Host,
			lo.Ternary(asset.Port != "", asset.Port, icons.GetDefaultPort(asset.Protocol)),
			asset.Command,
		}
	}
	m.table.SetRows(rows)

	// Update table height to show all filtered results when possible
	// Table height includes header + separator + data rows
	newHeight := len(m.filteredAssets) + 2
	if newHeight < 3 {
		newHeight = 3 // At least header + separator + 1 data row
	}

	// Calculate max available height
	maxHeight := m.height - 10
	if maxHeight < 5 {
		maxHeight = 5 // Minimum usable height
	}

	// Use actual row count for small sets, cap for large sets
	if newHeight > maxHeight {
		newHeight = maxHeight
	}
	m.table.SetHeight(newHeight)
}

func (m Model) renderHelp() string {
	if !m.showHelp {
		return ""
	}

	var helpItems []string
	if m.filterModel.Active() {
		// Show filter-specific help
		helpItems = []string{
			"type to filter",
			"↑/↓ navigate",
			"enter connect",
			"esc cancel",
		}
	} else {
		// Show normal help
		helpItems = []string{
			"↑/↓ navigate",
			"enter connect",
			"/ filter",
			"esc back",
			"pgup/pgdn scroll",
			"g/G top/bottom",
		}
		if m.filter != "" {
			helpItems[2] = "/ clear filter"
		}
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8c8c8c")). // Secondary text
		Padding(1, 0, 0, 0).
		Render(strings.Join(helpItems, " • "))
}

// Commands

type ConnectMsg struct {
	Asset Asset
}

func connectCmd(asset Asset) tea.Cmd {
	return func() tea.Msg {
		return ConnectMsg{Asset: asset}
	}
}

type BackMsg struct{}

func backCmd() tea.Cmd {
	return func() tea.Msg {
		return BackMsg{}
	}
}

// GetSelectedAsset returns the currently selected asset
func (m Model) GetSelectedAsset() *Asset {
	if m.table.SelectedRow() != nil && m.table.Cursor() < len(m.filteredAssets) {
		return &m.filteredAssets[m.table.Cursor()]
	}
	return nil
}

// SetFocus sets the focus state of the table
func (m *Model) SetFocus(focused bool) {
	m.focused = focused
	m.table.SetCursor(0)
}

// IsFilterActive returns whether the filter is active
func (m Model) IsFilterActive() bool {
	return m.filterModel.Active()
}

// NewRecentSessions creates a new asset list table from recent sessions
func NewRecentSessions(sessions []*model.Session, combines map[string][3]int, width, height int) Model {
	// Convert sessions to asset list with last login time
	assetList := make([]Asset, 0, len(sessions))
	for _, session := range sessions {
		// Parse protocol
		protocolParts := strings.Split(session.Protocol, ":")
		protocol := protocolParts[0]
		port := ""
		if len(protocolParts) > 1 {
			port = protocolParts[1]
		}

		// Parse asset name
		assetName := session.AssetInfo
		if parts := strings.Split(assetName, "("); len(parts) > 0 {
			assetName = strings.TrimSpace(parts[0])
		}

		// Parse account name
		userName := session.AccountInfo
		if parts := strings.Split(userName, "("); len(parts) > 0 {
			userName = strings.TrimSpace(parts[0])
		}

		// Build command string
		cmd := fmt.Sprintf("%s %s@%s", protocol, userName, assetName)
		if port != "" && port != lo.Ternary(protocol == "ssh", "22", lo.Ternary(protocol == "telnet", "23", lo.Ternary(protocol == "redis", "6379", lo.Ternary(protocol == "mysql", "3306", lo.Ternary(protocol == "postgresql", "5432", lo.Ternary(protocol == "mongodb", "27017", port)))))) {
			cmd = fmt.Sprintf("%s:%s", cmd, port)
		}

		// Look up the asset info from combines map if available
		info := [3]int{session.AccountId, session.AssetId, lo.Ternary(port != "", lo.Must(strconv.Atoi(port)), 0)}
		if val, ok := combines[cmd]; ok {
			info = val
		}

		assetList = append(assetList, Asset{
			Protocol:  protocol,
			Command:   cmd,
			User:      userName,
			Host:      assetName,
			Port:      port,
			Info:      info,
			LastLogin: &session.CreatedAt, // Store last login time
		})
	}

	// Create table columns with Last Login column
	columns := []table.Column{
		{Title: "Protocol", Width: 12},
		{Title: "User", Width: 15},
		{Title: "Host", Width: 25},
		{Title: "Port", Width: 8},
		{Title: "Last Login", Width: 18},
		{Title: "Command", Width: 35},
	}

	// Create table rows
	rows := make([]table.Row, len(assetList))
	for i, asset := range assetList {
		icon := icons.GetProtocolIcon(asset.Protocol)
		timeAgo := formatTimeAgo(*asset.LastLogin)
		rows[i] = table.Row{
			fmt.Sprintf("%s %s", icon, strings.ToUpper(asset.Protocol)),
			asset.User,
			asset.Host,
			lo.Ternary(asset.Port != "", asset.Port, icons.GetDefaultPort(asset.Protocol)),
			timeAgo,
			asset.Command,
		}
	}

	// Calculate viewport height
	viewportHeight := len(assetList) + 2
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	maxViewportHeight := height - 10
	if maxViewportHeight < 5 {
		maxViewportHeight = 5
	}

	if viewportHeight > maxViewportHeight {
		viewportHeight = maxViewportHeight
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(viewportHeight),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#b1c9ff")).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("#2f54eb"))
	s.Selected = selectedStyle
	t.SetStyles(s)

	return Model{
		table:          t,
		assets:         assetList,
		filteredAssets: assetList,
		filterModel:    NewFilter(),
		width:          width,
		height:         height,
		focused:        false,
		keyMap:         DefaultTableKeyMap,
		showHelp:       true,
		isRecent:       true, // Mark as recent sessions table
	}
}

// formatTimeAgo formats time as relative time
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d min%s ago", minutes, lo.Ternary(minutes > 1, "s", ""))
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, lo.Ternary(hours > 1, "s", ""))
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, lo.Ternary(days > 1, "s", ""))
	} else {
		return t.Format("2006-01-02 15:04")
	}
}
