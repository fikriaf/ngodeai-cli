package dialog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ThemeSelectedMsg is sent when a theme is selected
type ThemeSelectedMsg struct {
	Name string
}

// ThemeCloseMsg is sent when the dialog is closed
type ThemeCloseMsg struct{}

// ThemeItem represents a theme option
type ThemeItem struct {
	Name    string
	Preview []string // Color hex values for preview
}

// ThemePicker is the theme selection dialog
type ThemePicker struct {
	themes  []ThemeItem
	cursor  int
	current string
}

// NewThemePicker creates a new theme picker
func NewThemePicker(themes []ThemeItem, current string) ThemePicker {
	tp := ThemePicker{themes: themes, current: current}
	for i, t := range themes {
		if t.Name == current {
			tp.cursor = i
			break
		}
	}
	return tp
}

func (t ThemePicker) Init() tea.Cmd { return nil }

func (t ThemePicker) Update(msg tea.Msg) (ThemePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return t, func() tea.Msg { return ThemeCloseMsg{} }
		case "enter":
			if t.cursor < len(t.themes) {
				return t, func() tea.Msg { return ThemeSelectedMsg{Name: t.themes[t.cursor].Name} }
			}
		case "up", "k":
			if t.cursor > 0 {
				t.cursor--
			}
		case "down", "j":
			if t.cursor < len(t.themes)-1 {
				t.cursor++
			}
		}
	}
	return t, nil
}

func (t ThemePicker) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Select Theme")

	var b strings.Builder
	b.WriteString(title + "\n\n")

	for i, theme := range t.themes {
		prefix := "  "
		if i == t.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("> ")
		}

		current := ""
		if theme.Name == t.current {
			current = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(" (active)")
		}

		// Color preview swatches
		var swatches string
		for _, color := range theme.Preview {
			swatches += lipgloss.NewStyle().
				Background(lipgloss.Color(color)).
				Foreground(lipgloss.Color(color)).
				Render("██")
		}

		b.WriteString(prefix + theme.Name + current + " " + swatches + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("j/k navigate · enter select · esc close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Render(b.String())
}
