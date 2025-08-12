package colors

import "github.com/charmbracelet/lipgloss"

// Primary color palette - Professional blue theme
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#2f54eb") // Main brand color
	PrimaryColor2  = lipgloss.Color("#7f97fa") // Lighter primary
	PrimaryColor3  = lipgloss.Color("#ebeff8") // Very light background
	PrimaryColor4  = lipgloss.Color("#e1efff") // Light background
	PrimaryColor5  = lipgloss.Color("#f0f5ff") // Subtle background
	PrimaryColor6  = lipgloss.Color("#f9fbff") // Almost white
	PrimaryColor7  = lipgloss.Color("#f7f8fa") // Neutral light
	PrimaryColor8  = lipgloss.Color("#b1c9ff") // Light accent
	PrimaryColor9  = lipgloss.Color("#3F75FF") // Bright primary
	
	// Semantic colors
	SuccessColor = lipgloss.Color("#52c41a") // Green for success
	WarningColor = lipgloss.Color("#faad14") // Orange for warning
	ErrorColor   = lipgloss.Color("#f5222d") // Red for errors
	InfoColor    = lipgloss.Color("#1890ff") // Blue for info
	
	// Text colors (optimized for dark terminals)
	TextPrimary   = lipgloss.Color("#E0E0E0") // Light gray for main text
	TextSecondary = lipgloss.Color("#8c8c8c") // Medium gray for secondary text
	TextDisabled  = lipgloss.Color("#5c5c5c") // Dark gray for disabled text
	TextInverse   = lipgloss.Color("#ffffff") // White text on dark bg
	
	// Protocol-specific colors (using primary palette)
	SSHColor        = PrimaryColor9  // Bright blue for SSH
	MySQLColor      = PrimaryColor   // Deep blue for MySQL
	RedisColor      = lipgloss.Color("#9C27B0") // Purple for Redis
	MongoDBColor    = lipgloss.Color("#4DB33D") // Keep MongoDB brand green
	PostgreSQLColor = PrimaryColor2  // Light blue for PostgreSQL
	TelnetColor     = PrimaryColor8  // Soft blue for Telnet
)

// Styles using the color palette
var (
	// Text styles
	PrimaryStyle   = lipgloss.NewStyle().Foreground(PrimaryColor)
	SecondaryStyle = lipgloss.NewStyle().Foreground(PrimaryColor2)
	AccentStyle    = lipgloss.NewStyle().Foreground(PrimaryColor9).Bold(true)
	
	// Status styles
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	WarningStyle = lipgloss.NewStyle().Foreground(WarningColor)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	InfoStyle    = lipgloss.NewStyle().Foreground(InfoColor)
	
	// UI element styles
	TitleStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Underline(true)
	
	SubtitleStyle = lipgloss.NewStyle().
		Foreground(PrimaryColor2).
		Bold(true)
	
	HintStyle = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Italic(true)
	
	HighlightStyle = lipgloss.NewStyle().
		Foreground(TextInverse).
		Background(PrimaryColor9).
		Bold(true)
	
	// Banner gradient styles
	GradientStyle1 = lipgloss.NewStyle().Foreground(PrimaryColor9)
	GradientStyle2 = lipgloss.NewStyle().Foreground(PrimaryColor)
	GradientStyle3 = lipgloss.NewStyle().Foreground(PrimaryColor2)
)

// GetProtocolColor returns the appropriate color for each protocol
func GetProtocolColor(protocol string) lipgloss.Color {
	switch protocol {
	case "ssh":
		return SSHColor
	case "mysql":
		return MySQLColor
	case "redis":
		return RedisColor
	case "mongodb":
		return MongoDBColor
	case "postgresql":
		return PostgreSQLColor
	case "telnet":
		return TelnetColor
	default:
		return TextSecondary
	}
}

// GetProtocolStyle returns a styled protocol name
func GetProtocolStyle(protocol string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(GetProtocolColor(protocol)).
		Bold(true)
}