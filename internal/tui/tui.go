package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/app"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	// streamingCursor is appended to the last assistant message while streaming
	// to give the user a visual "typing" indicator.
	streamingCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Render(" ▌")
)

// Model is the main TUI model
type Model struct {
	app       *app.App
	viewport  viewport.Model
	textarea  textarea.Model
	messages  []ChatMessage
	ready     bool
	loading   bool
	width     int
	height    int
	sessionID string
	err       error

	// Streaming state
	streaming       bool
	streamContentCh <-chan string
	streamErrCh     <-chan error
}

// ChatMessage represents a displayed message
type ChatMessage struct {
	Role    string
	Content string
}

// New creates a new TUI model
func New(a *app.App) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything... (Ctrl+C to quit)"
	ta.Focus()
	ta.Prompt = "\u2503 "
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	return Model{
		app:      a,
		textarea: ta,
		viewport: vp,
		messages: []ChatMessage{
			{Role: "system", Content: "Welcome to NgodeAI CLI! Type your question and press Enter."},
		},
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit

		case "enter":
			if m.textarea.Value() != "" && !m.loading {
				content := m.textarea.Value()
				m.textarea.Reset()
				m.messages = append(m.messages, ChatMessage{Role: "user", Content: content})
				m.loading = true
				m.viewport.GotoBottom()

				// Prefer streaming; fall back to synchronous if agent is unavailable
				if m.app.Agent != nil {
					m.streaming = true
					return m, m.startStreaming(content)
				}
				return m, m.sendMessage(content)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 5
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width-4, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.textarea.SetWidth(msg.Width - 4)

	// ── Streaming messages ────────────────────────────────────────
	case streamSetupMsg:
		// Stream channels are ready; add a placeholder assistant message
		if msg.err != nil {
			// Setup failed → fall back to non-streaming
			m.streaming = false
			return m, m.sendMessage(msg.content)
		}
		m.sessionID = msg.sessionID
		m.streamContentCh = msg.contentCh
		m.streamErrCh = msg.errCh
		m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: ""})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, m.waitForStreamChunk()

	case streamChunkMsg:
		// Accumulate the chunk into the current (last) assistant message
		if len(m.messages) > 0 {
			last := &m.messages[len(m.messages)-1]
			if last.Role == "assistant" {
				last.Content += string(msg)
			}
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, m.waitForStreamChunk()

	case streamDoneMsg:
		m.streaming = false
		m.loading = false
		m.streamContentCh = nil
		m.streamErrCh = nil
		if msg.err != nil {
			errText := fmt.Sprintf("\n\n⚠ Error: %v", msg.err)
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
				m.messages[len(m.messages)-1].Content += errText
			} else {
				m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: fmt.Sprintf("Error: %v", msg.err)})
			}
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	// ── Non-streaming fallback ────────────────────────────────────
	case responseMsg:
		m.loading = false
		m.sessionID = msg.sessionID
		if msg.err != nil {
			m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: fmt.Sprintf("Error: %v", msg.err)})
		} else {
			m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: msg.content})
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := titleStyle.Render("NgodeAI CLI v0.2.0")

	status := ""
	if m.streaming {
		status = statusStyle.Render("Streaming…")
	} else if m.loading {
		status = statusStyle.Render("Thinking...")
	} else if m.app.Agent != nil {
		modelInfo := m.app.Agent.Provider().Model()
		status = statusStyle.Render(fmt.Sprintf("Model: %s (%s)", modelInfo.Name, modelInfo.Provider))
	}

	footer := m.textarea.View()

	separator := strings.Repeat("-", max(m.width, 1))

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
		header,
		status,
		m.viewport.View(),
		separator,
		footer,
	)
}

func (m Model) renderMessages() string {
	var b strings.Builder
	for i, msg := range m.messages {
		switch msg.Role {
		case "user":
			b.WriteString(userStyle.Render("You"))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Padding(0, 1).MaxWidth(80).Render(msg.Content))
		case "assistant":
			b.WriteString(assistantStyle.Render("NgodeAI"))
			b.WriteString("\n")
			content := msg.Content
			// Show a typing cursor on the last assistant message while streaming
			if m.streaming && i == len(m.messages)-1 {
				content += streamingCursor
			}
			b.WriteString(lipgloss.NewStyle().Padding(0, 1).MaxWidth(80).Render(content))
		case "system":
			b.WriteString(statusStyle.Render(msg.Content))
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── Streaming tea.Msg types ──────────────────────────────────────────────────

// streamSetupMsg is sent once the streaming goroutine has created the session
// and obtained the content/error channels from the agent.  On failure the
// embedded err is non-nil and content carries the original user text so the
// TUI can fall back to the synchronous path.
type streamSetupMsg struct {
	contentCh <-chan string
	errCh     <-chan error
	sessionID string
	content   string // original user text (for fallback)
	err       error  // non-nil if setup failed
}

// streamChunkMsg carries a single content delta from the streaming response.
type streamChunkMsg string

// streamDoneMsg signals that the stream has finished (successfully or with an
// error).
type streamDoneMsg struct {
	err error
}

// ── Streaming commands ───────────────────────────────────────────────────────

// startStreaming kicks off the agent's StreamRun in a background goroutine
// and returns a streamSetupMsg once the channels are ready.
func (m Model) startStreaming(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Create session if needed
		sessionID := m.sessionID
		if sessionID == "" {
			sess, err := m.app.Sessions.Create("TUI Session")
			if err != nil {
				return streamSetupMsg{content: content, err: err}
			}
			sessionID = sess.ID
		}

		// Start streaming
		contentCh, errCh := m.app.Agent.StreamRun(ctx, sessionID, content)
		return streamSetupMsg{
			contentCh: contentCh,
			errCh:     errCh,
			sessionID: sessionID,
			content:   content,
		}
	}
}

// waitForStreamChunk returns a tea.Cmd that blocks until the next content
// delta or a terminal error/close arrives from the streaming channels.
func (m Model) waitForStreamChunk() tea.Cmd {
	contentCh := m.streamContentCh
	errCh := m.streamErrCh
	if contentCh == nil && errCh == nil {
		return nil
	}

	return func() tea.Msg {
		// Prioritise content over errors so we never lose a chunk when
		// both channels become ready simultaneously.
		if contentCh != nil {
			select {
			case chunk, ok := <-contentCh:
				if !ok {
					// Content channel closed – drain any deferred error
					if errCh != nil {
						if err, ok := <-errCh; ok && err != nil {
							return streamDoneMsg{err: err}
						}
					}
					return streamDoneMsg{}
				}
				return streamChunkMsg(chunk)
			default:
			}
		}

		// Nothing ready yet – block on both
		select {
		case chunk, ok := <-contentCh:
			if !ok {
				if errCh != nil {
					if err, ok := <-errCh; ok && err != nil {
						return streamDoneMsg{err: err}
					}
				}
				return streamDoneMsg{}
			}
			return streamChunkMsg(chunk)

		case err, ok := <-errCh:
			if !ok {
				return streamDoneMsg{}
			}
			return streamDoneMsg{err: err}
		}
	}
}

// ── Non-streaming fallback ───────────────────────────────────────────────────

// Messages for async operations (non-streaming fallback)
type responseMsg struct {
	content   string
	sessionID string
	err       error
}

// sendMessage sends a message to the LLM agent (non-streaming fallback)
func (m Model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Create session if needed
		sessionID := m.sessionID
		if sessionID == "" {
			sess, err := m.app.Sessions.Create("TUI Session")
			if err != nil {
				return responseMsg{err: err}
			}
			sessionID = sess.ID
		}

		// Run the agent (non-streaming)
		response, err := m.app.Agent.Run(ctx, sessionID, content)
		if err != nil {
			return responseMsg{err: err, sessionID: sessionID}
		}

		return responseMsg{content: response, sessionID: sessionID}
	}
}
