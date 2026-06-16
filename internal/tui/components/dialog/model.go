package dialog

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModelItem represents a selectable model
type ModelItem struct {
	Provider      string
	Name          string
	ContextWindow int64
}

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Provider string
	Name     string
}

// ModelCloseMsg is sent when the dialog is closed
type ModelCloseMsg struct{}

// ModelSelector is the model selection dialog
type ModelSelector struct {
	models   []ModelItem
	cursor   int
	width    int
	height   int
	filter   string
	filtered []int
}

// NewModelSelector creates a new model selector
func NewModelSelector(models []ModelItem) ModelSelector {
	filtered := make([]int, len(models))
	for i := range models {
		filtered[i] = i
	}
	return ModelSelector{
		models:   models,
		filtered: filtered,
	}
}

func (m ModelSelector) Init() tea.Cmd { return nil }

func (m ModelSelector) Update(msg tea.Msg) (ModelSelector, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return ModelCloseMsg{} }
		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				item := m.models[m.filtered[m.cursor]]
				return m, func() tea.Msg {
					return ModelSelectedMsg{Provider: item.Provider, Name: item.Name}
				}
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
		default:
			if len(msg.String()) == 1 {
				m.filter += msg.String()
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m *ModelSelector) applyFilter() {
	m.filtered = m.filtered[:0]
	for i, model := range m.models {
		if m.filter == "" || strings.Contains(strings.ToLower(model.Name), strings.ToLower(m.filter)) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = 0
}

func (m ModelSelector) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Select Model")

	var b strings.Builder
	b.WriteString(title + "\n\n")

	if m.filter != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Filter: "+m.filter) + "\n")
	}

	maxItems := 15
	start := 0
	if m.cursor >= maxItems {
		start = m.cursor - maxItems + 1
	}

	for i := start; i < len(m.filtered) && i < start+maxItems; i++ {
		item := m.models[m.filtered[i]]
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("> ")
		}

		ctxStr := ""
		if item.ContextWindow > 0 {
			ctxStr = fmt.Sprintf(" (%dk ctx)", item.ContextWindow/1000)
		}

		line := fmt.Sprintf("%s%s%s %s",
			prefix,
			lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("["+item.Provider+"]"),
			item.Name,
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(ctxStr),
		)
		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("j/k navigate · enter select · esc close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Render(b.String())
}

// SetSize updates the dialog dimensions
func (m *ModelSelector) SetSize(w, h int) {
	m.width = w
	m.height = h
}
