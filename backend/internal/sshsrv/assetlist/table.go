package assetlist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"
	
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
	Protocol string
	Command  string
	User     string
	Host     string
	Port     string
	Info     [3]int // [accountId, assetId, port]
}

// Model represents the asset list table model
type Model struct {
	table         table.Model
	assets        []Asset
	filteredAssets []Asset
	filter        string
	width         int
	height        int
	focused       bool
	keyMap        TableKeyMap
	showHelp      bool
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
		{Title: "Protocol", Width: 12},  // Increased for emoji + text
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
	
	// Create table
	// Calculate viewport height based on terminal size
	// We need to account for:
	// - Title: 1 line
	// - Table header: 3 lines (with borders)
	// - Help text: 2 lines
	// - Borders and padding: 4 lines
	// Total overhead: ~10 lines
	viewportHeight := len(assetList) // Show all rows if possible
	maxViewportHeight := height - 10
	if maxViewportHeight > 5 && viewportHeight > maxViewportHeight {
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
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filter != "" && msg.Type == tea.KeyEscape {
			// Clear filter
			m.filter = ""
			m.updateFilter()
			return m, nil
		}
		
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
			// Start filtering mode
			return m, startFilterCmd()
			
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
		// Account for overhead (title, help, borders, etc.)
		newHeight := len(m.filteredAssets)
		maxHeight := msg.Height - 10
		if maxHeight > 5 && newHeight > maxHeight {
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
	// Title
	title := titleStyle.Render("🗂️  Available Assets")
	
	// Filter indicator
	filterInfo := ""
	if m.filter != "" {
		filterInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8c8c8c")). // Secondary text
			Render(fmt.Sprintf(" (filtered: %s)", m.filter))
	}
	
	// Asset count
	count := fmt.Sprintf("%d assets", len(m.filteredAssets))
	if m.filter != "" && len(m.filteredAssets) != len(m.assets) {
		count = fmt.Sprintf("%d of %d assets", len(m.filteredAssets), len(m.assets))
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
	
	// Build the final view with proper spacing
	var output strings.Builder
	output.WriteString(header)
	output.WriteString("\n")
	output.WriteString(tableBox)
	output.WriteString("\n")
	output.WriteString(help)
	
	return output.String()
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
	
	// Update table height if needed after filtering
	newHeight := len(m.filteredAssets)
	maxHeight := m.height - 10
	if maxHeight > 5 && newHeight > maxHeight {
		newHeight = maxHeight
	}
	m.table.SetHeight(newHeight)
}

func (m Model) renderHelp() string {
	if !m.showHelp {
		return ""
	}
	
	helpItems := []string{
		"↑/↓ navigate",
		"enter connect",
		"/ filter",
		"esc back",
		"pgup/pgdn scroll",
		"g/G top/bottom",
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

type backMsg struct{}

func backCmd() tea.Cmd {
	return func() tea.Msg {
		return backMsg{}
	}
}

type startFilterMsg struct{}

func startFilterCmd() tea.Cmd {
	return func() tea.Msg {
		return startFilterMsg{}
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