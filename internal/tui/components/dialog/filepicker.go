package dialog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileSelectedMsg is sent when a file is selected
type FileSelectedMsg struct {
	Path string
}

// FileCloseMsg is sent when the dialog is closed
type FileCloseMsg struct{}

// FilePicker is the file browser dialog
type FilePicker struct {
	dir     string
	files   []os.DirEntry
	cursor  int
	wd      string
}

// NewFilePicker creates a new file picker
func NewFilePicker(dir string) FilePicker {
	if dir == "" {
		dir, _ = os.Getwd()
	}

	fp := FilePicker{dir: dir}
	fp.loadFiles()
	return fp
}

func (f *FilePicker) loadFiles() {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return
	}

	// Filter hidden files and sort
	var filtered []os.DirEntry
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			filtered = append(filtered, e)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		// Directories first
		if filtered[i].IsDir() != filtered[j].IsDir() {
			return filtered[i].IsDir()
		}
		return filtered[i].Name() < filtered[j].Name()
	})

	// Backspace key handles ".." navigation

	f.files = filtered
	f.wd = f.dir
	f.cursor = 0
}

func (f FilePicker) Init() tea.Cmd { return nil }

func (f FilePicker) Update(msg tea.Msg) (FilePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return f, func() tea.Msg { return FileCloseMsg{} }
		case "enter":
			if len(f.files) > 0 && f.cursor < len(f.files) {
				entry := f.files[f.cursor]
				if entry.IsDir() {
					f.dir = filepath.Join(f.dir, entry.Name())
					f.loadFiles()
					return f, nil
				}
				return f, func() tea.Msg {
					return FileSelectedMsg{Path: filepath.Join(f.dir, entry.Name())}
				}
			}
		case "backspace":
			// Go up
			parent := filepath.Dir(f.dir)
			if parent != f.dir {
				f.dir = parent
				f.loadFiles()
			}
		case "up", "k":
			if f.cursor > 0 {
				f.cursor--
			}
		case "down", "j":
			if f.cursor < len(f.files)-1 {
				f.cursor++
			}
		}
	}
	return f, nil
}

func (f FilePicker) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("Select File")
	path := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(f.dir)

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(path + "\n\n")

	maxItems := 20
	start := 0
	if f.cursor >= maxItems {
		start = f.cursor - maxItems + 1
	}

	for i := start; i < len(f.files) && i < start+maxItems; i++ {
		entry := f.files[i]
		prefix := "  "
		if i == f.cursor {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("> ")
		}

		icon := "📄 "
		nameStyle := lipgloss.NewStyle()
		if entry.IsDir() {
			icon = "📁 "
			nameStyle = nameStyle.Foreground(lipgloss.Color("86")).Bold(true)
		}

		b.WriteString(prefix + icon + nameStyle.Render(entry.Name()) + "\n")
	}

	if len(f.files) == 0 {
		b.WriteString("  (empty directory)\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("j/k navigate · enter open/select · backspace up · esc close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Render(b.String())
}
