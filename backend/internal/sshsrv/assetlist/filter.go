package assetlist

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/veops/oneterm/internal/sshsrv/colors"
)

// FilterModel represents the filter input model
type FilterModel struct {
	textInput textinput.Model
	active    bool
}

// NewFilter creates a new filter model
func NewFilter() FilterModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.CharLimit = 50
	ti.SetWidth(20) // Reduce width to avoid jumping
	ti.Prompt = ">" // Simple prompt
	styles := ti.Styles()
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(colors.PrimaryColor)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(colors.TextPrimary)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(colors.TextSecondary)
	ti.SetStyles(styles)

	return FilterModel{
		textInput: ti,
		active:    false,
	}
}

// Active returns whether the filter is active
func (f FilterModel) Active() bool {
	return f.active
}

// Value returns the current filter value
func (f FilterModel) Value() string {
	return f.textInput.Value()
}

func (f *FilterModel) SetWidth(width int) {
	f.textInput.SetWidth(max(1, width))
}

// SetActive sets the filter active state
func (f *FilterModel) SetActive(active bool) tea.Cmd {
	f.active = active
	if active {
		f.textInput.Reset() // Clear any previous input
		return f.textInput.Focus()
	} else {
		f.textInput.Blur()
		f.textInput.Reset()
	}
	return nil
}

// Update handles filter input updates
func (f FilterModel) Update(msg tea.Msg) (FilterModel, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	var cmd tea.Cmd
	f.textInput, cmd = f.textInput.Update(msg)
	return f, cmd
}

// View renders the filter input
func (f FilterModel) View() string {
	if !f.active {
		return ""
	}

	// Just return the textinput view as-is
	return f.textInput.View()
}
