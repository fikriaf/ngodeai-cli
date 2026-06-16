package theme

import "github.com/charmbracelet/lipgloss"

// CatppuccinTheme uses the Catppuccin Mocha palette
func CatppuccinTheme() Theme {
	return Theme{
		Name:         "catppuccin",
		Primary:      lipgloss.Color("#CBA6F7"), // Mauve
		Secondary:    lipgloss.Color("#94E2D5"), // Teal
		Error:        lipgloss.Color("#F38BA8"), // Red
		Warning:      lipgloss.Color("#FAB387"), // Peach
		Success:      lipgloss.Color("#A6E3A1"), // Green
		Text:         lipgloss.Color("#CDD6F4"), // Text
		TextMuted:    lipgloss.Color("#6C7086"), // Overlay0
		Background:   lipgloss.Color("#1E1E2E"), // Base
		DiffAdded:    lipgloss.Color("#A6E3A1"), // Green
		DiffRemoved:  lipgloss.Color("#F38BA8"), // Red
		UserMsg:      lipgloss.Color("#89B4FA"), // Blue
		AssistantMsg: lipgloss.Color("#CBA6F7"), // Mauve
	}
}
