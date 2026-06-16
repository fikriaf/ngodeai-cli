package theme

import "github.com/charmbracelet/lipgloss"

// DraculaTheme uses the Dracula color palette
func DraculaTheme() Theme {
	return Theme{
		Name:         "dracula",
		Primary:      lipgloss.Color("#BD93F9"), // Purple
		Secondary:    lipgloss.Color("#8BE9FD"), // Cyan
		Error:        lipgloss.Color("#FF5555"), // Red
		Warning:      lipgloss.Color("#FFB86C"), // Orange
		Success:      lipgloss.Color("#50FA7B"), // Green
		Text:         lipgloss.Color("#F8F8F2"), // Foreground
		TextMuted:    lipgloss.Color("#6272A4"), // Comment
		Background:   lipgloss.Color("#282A36"), // Background
		DiffAdded:    lipgloss.Color("#50FA7B"), // Green
		DiffRemoved:  lipgloss.Color("#FF5555"), // Red
		UserMsg:      lipgloss.Color("#8BE9FD"), // Cyan
		AssistantMsg: lipgloss.Color("#BD93F9"), // Purple
	}
}
