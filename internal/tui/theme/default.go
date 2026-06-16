package theme

import "github.com/charmbracelet/lipgloss"

// DefaultTheme is the built-in default color scheme
func DefaultTheme() Theme {
	return Theme{
		Name:         "default",
		Primary:      lipgloss.Color("#7C3AED"),
		Secondary:    lipgloss.Color("#06B6D4"),
		Error:        lipgloss.Color("#EF4444"),
		Warning:      lipgloss.Color("#F59E0B"),
		Success:      lipgloss.Color("#10B981"),
		Text:         lipgloss.Color("#F8FAFC"),
		TextMuted:    lipgloss.Color("#94A3B8"),
		Background:   lipgloss.Color("#0F172A"),
		DiffAdded:    lipgloss.Color("#22C55E"),
		DiffRemoved:  lipgloss.Color("#EF4444"),
		UserMsg:      lipgloss.Color("#38BDF8"),
		AssistantMsg: lipgloss.Color("#A78BFA"),
	}
}
