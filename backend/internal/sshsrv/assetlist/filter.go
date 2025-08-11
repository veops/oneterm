package assetlist

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ti.Width = 20  // Reduce width to avoid jumping
	ti.Prompt = ">" // Simple prompt
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colors.PrimaryColor)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colors.TextPrimary)
	
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

// SetActive sets the filter active state
func (f *FilterModel) SetActive(active bool) {
	f.active = active
	if active {
		f.textInput.Focus()
		f.textInput.Reset() // Clear any previous input
	} else {
		f.textInput.Blur()
		f.textInput.Reset()
	}
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