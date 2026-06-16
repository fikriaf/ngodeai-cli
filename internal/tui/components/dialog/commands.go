package dialog

import (
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/commands"
)

// ── Messages ────────────────────────────────────────────────────────────────

// CommandSelectedMsg is emitted when the user picks a command.
type CommandSelectedMsg struct {
	Command commands.Command
	Input   string // Additional input from the user
}

// CommandCloseMsg is emitted when the user dismisses the command palette.
type CommandCloseMsg struct{}

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	cmdDialogBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Width(72)

	cmdDialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				MarginBottom(1)

	cmdStatusBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				MarginTop(1)

	cmdSelectedTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	cmdSelectedDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	cmdNormalTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	cmdNormalDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
)

// ── List Item ───────────────────────────────────────────────────────────────

// commandItem wraps a commands.Command to satisfy the list.Item interface.
type commandItem struct {
	command commands.Command
}

func (i commandItem) Title() string       { return "/" + i.command.Name }
func (i commandItem) Description() string { return i.command.Description }
func (i commandItem) FilterValue() string { return i.command.Name + " " + i.command.Description }

// ── Custom Delegate ─────────────────────────────────────────────────────────

// commandDelegate renders command items with descriptions.
type commandDelegate struct{}

func (d commandDelegate) Height() int                             { return 2 }
func (d commandDelegate) Spacing() int                            { return 1 }
func (d commandDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d commandDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(commandItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = cmdSelectedTitleStyle
		descStyle = cmdSelectedDescStyle
	} else {
		titleStyle = cmdNormalTitleStyle
		descStyle = cmdNormalDescStyle
	}

	// Indicator arrow for selected item
	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("▸ ")
	}

	// Title line
	titleLine := titleStyle.Render(item.Title())

	// Description line (truncated)
	desc := item.Description()
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	descLine := descStyle.Render(desc)

	io.WriteString(w, indicator+titleLine+"\n"+indicator+descLine)
}

// ── Model ───────────────────────────────────────────────────────────────────

// CommandModel is the Command Palette Bubble Tea model.
type CommandModel struct {
	list        list.Model
	input       textinput.Model
	cmds        []commands.Command
	width       int
	height      int
	showInput   bool
	selectedCmd *commands.Command
	inputMode   bool // true when typing additional input
}

// NewCommandPalette creates a new Command Palette.
func NewCommandPalette(svc *commands.Service) CommandModel {
	delegate := commandDelegate{}
	l := list.New(nil, delegate, 68, 16)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Create text input for additional arguments
	ti := textinput.New()
	ti.Placeholder = "Enter additional context (optional)..."
	ti.CharLimit = 500
	ti.Width = 60

	// Load commands
	cmdList := svc.List()
	sort.Slice(cmdList, func(i, j int) bool {
		return cmdList[i].Name < cmdList[j].Name
	})

	items := make([]list.Item, len(cmdList))
	for i, cmd := range cmdList {
		items[i] = commandItem{command: cmd}
	}
	l.SetItems(items)

	return CommandModel{
		list:  l,
		input: ti,
		cmds:  cmdList,
	}
}

// SetSize adjusts the dialog dimensions.
func (m *CommandModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	dialogW := width - 8
	if dialogW > 80 {
		dialogW = 80
	}
	dialogH := height - 6
	if dialogH > 24 {
		dialogH = 24
	}
	if dialogW < 40 {
		dialogW = 40
	}
	if dialogH < 8 {
		dialogH = 8
	}

	m.list.SetSize(dialogW-6, dialogH-6)
}

// Init implements tea.Model.
func (m CommandModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m CommandModel) Update(msg tea.Msg) (CommandModel, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		// Handle input mode separately
		if m.inputMode {
			switch msg.String() {
			case "esc":
				// Go back to command selection
				m.inputMode = false
				m.input.SetValue("")
				return m, nil
			case "enter":
				// Submit with input
				if m.selectedCmd != nil {
					return m, func() tea.Msg {
						return CommandSelectedMsg{
							Command: *m.selectedCmd,
							Input:   m.input.Value(),
						}
					}
				}
			case "ctrl+c":
				return m, func() tea.Msg { return CommandCloseMsg{} }
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CommandCloseMsg{} }

		case "enter":
			if i, ok := m.list.SelectedItem().(commandItem); ok {
				cmd := i.command
				// Check if command needs input
				if strings.Contains(cmd.Template, "{{input}}") ||
					strings.Contains(cmd.Template, "{{query}}") ||
					strings.Contains(cmd.Template, "{{text}}") {
					// Switch to input mode
					m.selectedCmd = &cmd
					m.inputMode = true
					m.input.Focus()
					return m, textinput.Blink
				}
				// No input needed, execute immediately
				return m, func() tea.Msg {
					return CommandSelectedMsg{Command: cmd}
				}
			}

		case "tab":
			// Switch to input mode for current selection
			if i, ok := m.list.SelectedItem().(commandItem); ok {
				cmd := i.command
				m.selectedCmd = &cmd
				m.inputMode = true
				m.input.Focus()
				return m, textinput.Blink
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m CommandModel) View() string {
	title := cmdDialogTitleStyle.Render("⚡  Command Palette")

	var content string
	if m.inputMode && m.selectedCmd != nil {
		// Show input mode
		cmdTitle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Render("/" + m.selectedCmd.Name)

		desc := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(m.selectedCmd.Description)

		inputLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Render("Additional input:")

		inputView := m.input.View()

		status := cmdStatusBarStyle.Render("enter: execute · esc: back · ctrl+c: cancel")

		content = cmdDialogBoxStyle.Render(
			title + "\n\n" +
				cmdTitle + "\n" +
				desc + "\n\n" +
				inputLabel + "\n" +
				inputView + "\n\n" +
				status,
		)
	} else {
		// Show command list
		status := cmdStatusBarStyle.Render("enter: select · tab: add input · esc: cancel · /: filter")
		content = cmdDialogBoxStyle.Width(m.list.Width() + 4).Render(
			title + "\n\n" + m.list.View() + "\n\n" + status,
		)
	}

	// Center the dialog
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// HasCommands returns true if there are commands available
func (m CommandModel) HasCommands() bool {
	return len(m.cmds) > 0
}
