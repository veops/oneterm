package assetlist

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		key.WithKeys("end", "G", "shift+g"),
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
	search    string
	Info      [3]int     // [accountId, assetId, port]
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
	entries := make([]Asset, 0, len(assets))
	for command, info := range assets {
		protocol, address, _ := strings.Cut(command, " ")
		user, host, _ := strings.Cut(address, "@")
		port := strconv.Itoa(info[2])
		host = strings.TrimSuffix(host, ":"+port)
		entries = append(entries, Asset{Protocol: protocol, Command: command, User: user, Host: host, Port: port, Info: info})
	}
	return NewConnections(entries, width, height)
}

func NewConnections(entries []Asset, width, height int) Model {
	return newTable(entries, width, height, false)
}

func newTable(entries []Asset, width, height int, recent bool) Model {
	entries = append([]Asset(nil), entries...)
	if !recent {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Command < entries[j].Command })
	}
	for i := range entries {
		entries[i].search = strings.ToLower(entries[i].Command + "\x00" + entries[i].Host + "\x00" + entries[i].User + "\x00" + entries[i].Protocol)
	}
	m := Model{assets: entries, width: width, height: height, isRecent: recent, filterModel: NewFilter(), keyMap: DefaultTableKeyMap, showHelp: true}
	m.table = table.New(table.WithColumns(m.calculateColumnWidths(width, recent)), table.WithFocused(true), table.WithHeight(5))
	styles := table.DefaultStyles()
	styles.Header = styles.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#b1c9ff")).BorderBottom(true).Bold(true).Foreground(lipgloss.Color("#2f54eb"))
	styles.Selected = selectedStyle
	m.table.SetStyles(styles)
	m.updateFilter()
	m.resize(width, height)
	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.resize(size.Width, size.Height)
		var cmd tea.Cmd
		m.filterModel, cmd = m.filterModel.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	var cmds []tea.Cmd
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && !m.canShowTable() && key.Matches(keyMsg, m.keyMap.Enter) {
		return m, nil
	}

	// Handle filter input first if active
	if m.filterModel.Active() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.Code {
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
			if filter := m.filterModel.Value(); filter != m.filter {
				m.filter = filter
				m.updateFilter()
				m.table.GotoTop()
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:

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
				cmd = m.filterModel.SetActive(true)
				// Initialize filter to empty to prepare for input
				m.filter = ""
			}
			return m, cmd

		case key.Matches(msg, m.keyMap.Up),
			key.Matches(msg, m.keyMap.Down),
			key.Matches(msg, m.keyMap.PageUp),
			key.Matches(msg, m.keyMap.PageDown),
			key.Matches(msg, m.keyMap.Home),
			key.Matches(msg, m.keyMap.End):
			// Let the table handle all navigation keys
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		default:
			// For any other key messages, let the table handle them
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}

	}

	return m, cmd
}

// View renders the table
func (m Model) View() string {
	if !m.canShowTable() {
		return lipgloss.NewStyle().MaxWidth(max(1, m.width)).MaxHeight(max(1, m.height)).Render("Resize terminal to view assets")
	}
	return m.headerView() + "\n" + baseStyle.Render(m.table.View()) + "\n" + m.renderHelp()
}

func (m Model) headerView() string {
	title := titleStyle.Render(lo.Ternary(m.isRecent, "Recent Sessions", "Available Assets"))

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

	return lipgloss.NewStyle().MaxWidth(max(1, m.width-2)).Render(lipgloss.JoinHorizontal(lipgloss.Left, title, filterInfo, " ", countStyle))
}

func (m *Model) resize(width, height int) {
	m.width, m.height = max(1, width), max(1, height)
	m.updateColumnWidths()
	m.table.SetWidth(max(1, m.width-2))
	m.filterModel.SetWidth(min(20, max(1, m.width-32)))
	m.updateTableHeight()
}

func (m Model) availableTableHeight() int {
	return m.height - lipgloss.Height(m.headerView()) - lipgloss.Height(m.renderHelp()) - baseStyle.GetVerticalFrameSize()
}

func (m Model) canShowTable() bool {
	minimumWidth := 28
	if m.isRecent {
		minimumWidth = 33
	}
	return m.width >= minimumWidth && m.availableTableHeight() >= 3
}

func (m *Model) updateTableHeight() {
	m.table.SetHeight(min(max(3, len(m.filteredAssets)+2), max(3, m.availableTableHeight())))
}

// Helper functions

func (m *Model) updateFilter() {
	if m.filter == "" {
		m.filteredAssets = m.assets
	} else {
		filter := strings.ToLower(m.filter)
		m.filteredAssets = lo.Filter(m.assets, func(a Asset, _ int) bool {
			return strings.Contains(a.search, filter)
		})
	}

	// Update table rows - handle different formats for recent sessions vs assets
	rows := make([]table.Row, len(m.filteredAssets))

	for i, asset := range m.filteredAssets {
		protocolText := strings.ToUpper(asset.Protocol)
		port := lo.Ternary(asset.Port != "", asset.Port, icons.GetDefaultPort(asset.Protocol))

		if m.isRecent && asset.LastLogin != nil {
			// Recent sessions format with Last Login column
			timeAgo := formatTimeAgo(*asset.LastLogin)
			rows[i] = table.Row{
				protocolText,
				asset.User,
				asset.Host,
				port,
				timeAgo,
				asset.Command,
			}
		} else {
			// Regular assets format
			rows[i] = table.Row{
				protocolText,
				asset.User,
				asset.Host,
				port,
				asset.Command,
			}
		}
	}
	m.table.SetRows(rows)

	m.updateTableHeight()
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
		Width(max(1, m.width-2)).Render(strings.Join(helpItems, " • "))
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
func NewRecentSessions(sessions []*model.Session, combines map[string][3]int, width, height int, connections ...[]Asset) Model {
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
		number, _ := strconv.Atoi(port)
		if number == 0 {
			number, _ = strconv.Atoi(icons.GetDefaultPort(protocol))
		}
		info := [3]int{session.AccountId, session.AssetId, number}
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

	if len(connections) > 0 {
		type key struct {
			info     [3]int
			protocol string
		}
		current := make(map[key]Asset, len(connections[0]))
		for _, connection := range connections[0] {
			current[key{connection.Info, connection.Protocol}] = connection
		}
		available := assetList[:0]
		for _, recent := range assetList {
			if connection, ok := current[key{recent.Info, recent.Protocol}]; ok {
				connection.LastLogin = recent.LastLogin
				available = append(available, connection)
			}
		}
		assetList = available
	}
	return newTable(assetList, width, height, true)
}

// formatTimeAgo formats time as relative time
// calculateColumnWidths calculates reasonable column widths
func (m *Model) calculateColumnWidths(width int, recent bool) []table.Column {
	columns := []table.Column{{Title: "Protocol", Width: 12}, {Title: "User", Width: 12}, {Title: "Host", Width: 18}, {Title: "Port", Width: 5}}
	if recent {
		columns = append(columns, table.Column{Title: "Last Login", Width: 12})
	}
	columns = append(columns, table.Column{Title: "Command", Width: 30})
	total := 0
	for _, column := range columns {
		total += column.Width
	}
	available := max(len(columns), width-2-2*len(columns))
	minimum := []int{3, 3, 4, 5}
	if recent {
		minimum = append(minimum, 3)
	}
	minimum = append(minimum, 1)
	reserve := 0
	for _, size := range minimum {
		reserve += size
	}
	remaining := available
	for i := range columns {
		columns[i].Width = max(1, minimum[i]+max(0, available-reserve)*columns[i].Width/total)
		remaining -= columns[i].Width
	}
	columns[len(columns)-1].Width += max(0, remaining)
	return columns
}

// updateColumnWidths updates table column widths based on current terminal size
func (m *Model) updateColumnWidths() {
	columns := m.calculateColumnWidths(m.width, m.isRecent)
	m.table.SetColumns(columns)
}

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
