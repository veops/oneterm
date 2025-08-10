package icons

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/veops/oneterm/internal/sshsrv/colors"
)

// GetProtocolIcon returns just the icon character for each protocol
func GetProtocolIcon(protocol string) string {
	switch protocol {
	case "ssh":
		return "▶"
	case "mysql":
		return "◆"
	case "redis":
		return "⚡"
	case "mongodb":
		return "◉"
	case "postgresql":
		return "▣"
	case "telnet":
		return "◎"
	default:
		return "●"
	}
}

// GetStyledProtocolIcon returns a styled icon for each protocol
func GetStyledProtocolIcon(protocol string) string {
	icon := GetProtocolIcon(protocol)
	switch protocol {
	case "ssh":
		return lipgloss.NewStyle().Foreground(colors.SSHColor).Render(icon)
	case "mysql":
		return lipgloss.NewStyle().Foreground(colors.MySQLColor).Render(icon)
	case "redis":
		return lipgloss.NewStyle().Foreground(colors.RedisColor).Render(icon)
	case "mongodb":
		return lipgloss.NewStyle().Foreground(colors.MongoDBColor).Render(icon)
	case "postgresql":
		return lipgloss.NewStyle().Foreground(colors.PostgreSQLColor).Render(icon)
	case "telnet":
		return lipgloss.NewStyle().Foreground(colors.TelnetColor).Render(icon)
	default:
		return lipgloss.NewStyle().Foreground(colors.TextSecondary).Render(icon)
	}
}

// GetDefaultPort returns the default port for each protocol
func GetDefaultPort(protocol string) string {
	switch protocol {
	case "ssh":
		return "22"
	case "mysql":
		return "3306"
	case "redis":
		return "6379"
	case "mongodb":
		return "27017"
	case "postgresql":
		return "5432"
	case "telnet":
		return "23"
	default:
		return ""
	}
}