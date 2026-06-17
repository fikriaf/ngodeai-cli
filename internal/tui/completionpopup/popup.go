package completionpopup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/tui/autocomplete"
	"github.com/fikriaf/ngodeai-cli/internal/tui/theme"
)

// Model represents the completion popup state
type Model struct {
	Visible      bool
	Completions  []autocomplete.Completion
	SelectedIdx  int
	MaxVisible   int // Max items to show at once
}

// New creates a new completion popup
func New() Model {
	return Model{
		Visible:     false,
		Completions: nil,
		SelectedIdx: 0,
		MaxVisible:  5,
	}
}

// Show displays the popup with given completions
func (m *Model) Show(completions []autocomplete.Completion) {
	if len(completions) == 0 {
		m.Hide()
		return
	}
	m.Visible = true
	m.Completions = completions
	m.SelectedIdx = 0
}

// Hide hides the popup
func (m *Model) Hide() {
	m.Visible = false
	m.Completions = nil
	m.SelectedIdx = 0
}

// Next selects the next completion
func (m *Model) Next() {
	if len(m.Completions) == 0 {
		return
	}
	m.SelectedIdx = (m.SelectedIdx + 1) % len(m.Completions)
}

// Prev selects the previous completion
func (m *Model) Prev() {
	if len(m.Completions) == 0 {
		return
	}
	m.SelectedIdx--
	if m.SelectedIdx < 0 {
		m.SelectedIdx = len(m.Completions) - 1
	}
}

// Selected returns the currently selected completion
func (m *Model) Selected() *autocomplete.Completion {
	if len(m.Completions) == 0 {
		return nil
	}
	return &m.Completions[m.SelectedIdx]
}

// Render renders the completion popup
func (m Model) Render(themeName string, width int) string {
	if !m.Visible || len(m.Completions) == 0 {
		return ""
	}

	t := theme.GetTheme(themeName)
	
	// Styling
	popupStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(0, 1)
	
	selectedStyle := lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background).
		Bold(true)
	
	normalStyle := lipgloss.NewStyle().
		Foreground(t.Text)
	
	descStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)
	
	// Calculate visible range
	start := 0
	end := len(m.Completions)
	if end > m.MaxVisible {
		// Center selection in visible range
		start = m.SelectedIdx - m.MaxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + m.MaxVisible
		if end > len(m.Completions) {
			end = len(m.Completions)
			start = end - m.MaxVisible
			if start < 0 {
				start = 0
			}
		}
	}
	
	var lines []string
	
	// Header
	header := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("Tab Completions")
	lines = append(lines, header)
	lines = append(lines, "")
	
	// Items
	for i := start; i < end; i++ {
		comp := m.Completions[i]
		
		// Truncate if too long
		text := comp.Text
		if len(text) > width/2 {
			text = text[:width/2-3] + "..."
		}
		
		var line string
		if i == m.SelectedIdx {
			// Selected item
			line = selectedStyle.Render("▶ " + text)
			if comp.Description != "" {
				line += " " + descStyle.Render(comp.Description)
			}
		} else {
			// Normal item
			line = normalStyle.Render("  " + text)
			if comp.Description != "" {
				line += " " + descStyle.Render(comp.Description)
			}
		}
		lines = append(lines, line)
	}
	
	// Footer hint
	if len(m.Completions) > m.MaxVisible {
		footer := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render("↑/↓ to navigate, Tab to complete, Esc to cancel")
		lines = append(lines, "")
		lines = append(lines, footer)
	}
	
	// Build popup
	popup := strings.Join(lines, "\n")
	return popupStyle.Render(popup)
}
