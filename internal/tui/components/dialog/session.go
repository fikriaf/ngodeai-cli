package dialog

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(72)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	selectedTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	selectedDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	normalTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	normalDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	dimDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

// ── Messages ────────────────────────────────────────────────────────────────

// SessionSelectedMsg is emitted when the user picks a session.
type SessionSelectedMsg struct {
	Session session.Session
}

// SessionCloseMsg is emitted when the user dismisses the browser.
type SessionCloseMsg struct{}

// sessionsLoadedMsg carries the result of the async session load.
type sessionsLoadedMsg struct {
	sessions []session.Session
	err      error
}

// ── List Item ───────────────────────────────────────────────────────────────

// sessionItem wraps a session.Session to satisfy the list.Item interface.
type sessionItem struct {
	session session.Session
}

func (i sessionItem) Title() string       { return i.session.Title }
func (i sessionItem) Description() string { return formatSessionMeta(i.session) }
func (i sessionItem) FilterValue() string { return i.session.Title }

// formatSessionMeta builds the secondary line: "12 msgs · 3m ago"
func formatSessionMeta(s session.Session) string {
	msgLabel := "msgs"
	if s.MessageCount == 1 {
		msgLabel = "msg"
	}
	return fmt.Sprintf("%d %s · %s", s.MessageCount, msgLabel, relativeTime(s.UpdatedAt))
}

// ── Custom Delegate ─────────────────────────────────────────────────────────

// sessionDelegate renders session items with rich metadata.
type sessionDelegate struct{}

func (d sessionDelegate) Height() int                             { return 2 }
func (d sessionDelegate) Spacing() int                            { return 1 }
func (d sessionDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d sessionDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(sessionItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = selectedTitleStyle
		descStyle = selectedDescStyle
	} else {
		titleStyle = normalTitleStyle
		descStyle = normalDescStyle
	}

	// Indicator arrow for selected item
	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("▸ ")
	}

	// Title line with cost badge
	titleLine := titleStyle.Render(item.session.Title)
	if item.session.Cost > 0 {
		costBadge := dimDescStyle.Render(fmt.Sprintf("  $%.4f", item.session.Cost))
		titleLine += costBadge
	}

	// Description line
	descLine := descStyle.Render(formatSessionMeta(item.session))

	fmt.Fprintf(w, "%s%s\n%s%s", indicator, titleLine, indicator, descLine)
}

// ── Model ───────────────────────────────────────────────────────────────────

// Model is the Session Browser Bubble Tea model.
type Model struct {
	list     list.Model
	svc      *session.Service
	width    int
	height   int
	err      error
	quitting bool
}

// New creates a new Session Browser.
// Pass a session.Service to load sessions from the database.
func New(svc *session.Service) Model {
	delegate := sessionDelegate{}
	l := list.New(nil, delegate, 68, 20)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = dialogTitleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	l.SetStatusBarItemName("session", "sessions")

	return Model{
		list: l,
		svc:  svc,
	}
}

// SetSize adjusts the dialog dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	dialogW := width - 8
	if dialogW > 80 {
		dialogW = 80
	}
	dialogH := height - 6
	if dialogH > 28 {
		dialogH = 28
	}
	if dialogW < 40 {
		dialogW = 40
	}
	if dialogH < 8 {
		dialogH = 8
	}

	m.list.SetSize(dialogW-6, dialogH-4)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return m.loadSessions()
}

// loadSessions returns a command that fetches sessions asynchronously.
func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.svc.List()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		items := make([]list.Item, len(msg.sessions))
		for i, s := range msg.sessions {
			items[i] = sessionItem{session: s}
		}
		cmd := m.list.SetItems(items)

		if len(msg.sessions) == 0 {
			m.list.NewStatusMessage(
				lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  No sessions yet. Start chatting to create one!"),
			)
		}
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.quitting = true
			return m, func() tea.Msg { return SessionCloseMsg{} }

		case "enter":
			if i, ok := m.list.SelectedItem().(sessionItem); ok {
				sess := i.session
				return m, func() tea.Msg { return SessionSelectedMsg{Session: sess} }
			}

		case "r":
			// Refresh sessions
			return m, m.loadSessions()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	if m.err != nil {
		return dialogBoxStyle.Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: "+m.err.Error()),
		)
	}

	title := dialogTitleStyle.Render("📂  Session Browser")
	status := statusBarStyle.Render("enter: select · esc: close · r: refresh · /: filter")

	content := fmt.Sprintf("%s\n\n%s\n\n%s", title, m.list.View(), status)

	dialog := dialogBoxStyle.Width(m.list.Width() + 4).Render(content)

	// Center the dialog in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// relativeTime formats a Unix timestamp as a human-friendly relative string.
func relativeTime(unixTs int64) string {
	if unixTs == 0 {
		return "never"
	}
	t := time.Unix(unixTs, 0)
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / (24 * 7))
		if weeks == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", weeks)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// truncate shortens a string to maxLen, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 2 {
		return s[:maxLen]
	}
	return strings.TrimSpace(s[:maxLen-1]) + "…"
}
