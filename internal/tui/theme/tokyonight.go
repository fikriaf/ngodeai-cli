package theme

import "github.com/charmbracelet/lipgloss"

// TokyoNightTheme uses the Tokyo Night color palette
func TokyoNightTheme() Theme {
	return Theme{
		Name:         "tokyonight",
		Primary:      lipgloss.Color("#BB9AF7"), // Purple
		Secondary:    lipgloss.Color("#7DCFFF"), // Cyan
		Error:        lipgloss.Color("#F7768E"), // Red
		Warning:      lipgloss.Color("#E0AF68"), // Yellow
		Success:      lipgloss.Color("#9ECE6A"), // Green
		Text:         lipgloss.Color("#C0CAF5"), // Foreground
		TextMuted:    lipgloss.Color("#565F89"), // Comment
		Background:   lipgloss.Color("#1A1B26"), // Background
		DiffAdded:    lipgloss.Color("#9ECE6A"), // Green
		DiffRemoved:  lipgloss.Color("#F7768E"), // Red
		UserMsg:      lipgloss.Color("#7AA2F7"), // Blue
		AssistantMsg: lipgloss.Color("#BB9AF7"), // Purple
	}
}
