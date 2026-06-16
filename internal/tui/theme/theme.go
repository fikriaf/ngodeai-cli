package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the TUI
type Theme struct {
	Name string

	// UI chrome
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Success   lipgloss.Color

	// Text
	Text      lipgloss.Color
	TextMuted lipgloss.Color

	// Background
	Background lipgloss.Color

	// Diff colors
	DiffAdded   lipgloss.Color
	DiffRemoved lipgloss.Color

	// Message colors
	UserMsg      lipgloss.Color
	AssistantMsg lipgloss.Color
}
