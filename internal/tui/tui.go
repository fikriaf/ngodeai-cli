package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/app"
	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/tui/components/dialog"
	"github.com/fikriaf/ngodeai-cli/internal/tui/slash"
	"github.com/fikriaf/ngodeai-cli/internal/tui/sidebar"
	"github.com/fikriaf/ngodeai-cli/internal/tui/statusbar"
	"github.com/fikriaf/ngodeai-cli/internal/tui/theme"
)

// activeDialog tracks which dialog overlay (if any) is currently shown.
type activeDialog int

const (
	noDialog    activeDialog = iota
	modelDialog              // Model selector (Ctrl+O)
	themeDialog              // Theme picker (Ctrl+T)
	fileDialog               // File attachment picker (Ctrl+F)
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

	// Dialog state
	activeDialog  activeDialog
	modelSelector dialog.ModelSelector
	themePicker   dialog.ThemePicker
	filePicker    dialog.FilePicker

	// Theme
	currentTheme string

	// Slash command registry
	slashRegistry *slash.Registry

	// UI state
	showSidebar    bool      // Toggle sidebar visibility (ctrl+b)
	startTime      time.Time // When streaming/thinking started
	responseTime   time.Duration // Last response time
	promptTokens   int64
	completionTokens int64
}

// ChatMessage represents a displayed message
type ChatMessage struct {
	Role    string
	Content string
}

// New creates a new TUI model
func New(a *app.App) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask anything... Type / for commands (Ctrl+O: model · Ctrl+T: theme · Ctrl+C: quit)"
	ta.Focus()
	ta.Prompt = "\u2503 "
	ta.CharLimit = 4096
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(80, 20)

	return Model{
		app:           a,
		textarea:      ta,
		viewport:      vp,
		messages: []ChatMessage{
			{Role: "system", Content: "Welcome to NgodeAI CLI! Type your question or /help for commands."},
		},
		currentTheme:  "default",
		slashRegistry: slash.NewRegistry(),
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

	// ── 1. Always handle dialog result messages ──────────────────────
	switch msg := msg.(type) {
	case dialog.ModelSelectedMsg:
		m.activeDialog = noDialog
		m.handleModelSelection(msg)
		return m, nil

	case dialog.ModelCloseMsg:
		m.activeDialog = noDialog
		return m, nil

	case dialog.ThemeSelectedMsg:
		m.activeDialog = noDialog
		m.currentTheme = msg.Name
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("✓ Theme changed to \"%s\"", msg.Name),
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil

	case dialog.ThemeCloseMsg:
		m.activeDialog = noDialog
		return m, nil

	case dialog.FileSelectedMsg:
		m.activeDialog = noDialog
		// Insert file path into the textarea
		current := m.textarea.Value()
		if current != "" {
			current += " "
		}
		m.textarea.SetValue(current + msg.Path)
		return m, nil

	case dialog.FileCloseMsg:
		m.activeDialog = noDialog
		return m, nil
	}

	// ── 2. Always handle window resize (for both main and dialog) ────
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsMsg.Width
		m.height = wsMsg.Height

		headerHeight := 3
		footerHeight := 5
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(wsMsg.Width-4, wsMsg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = wsMsg.Width - 4
			m.viewport.Height = wsMsg.Height - verticalMarginHeight
		}

		m.textarea.SetWidth(wsMsg.Width - 4)

		// Also resize active dialog
		switch m.activeDialog {
		case modelDialog:
			m.modelSelector.SetSize(wsMsg.Width, wsMsg.Height)
		case themeDialog:
			// themePicker doesn't need SetSize
		case fileDialog:
			// filePicker doesn't need SetSize
		}

		// If dialog is active, route WindowSizeMsg to it and return
		if m.activeDialog != noDialog {
			return m.routeToActiveDialog(msg)
		}
	}

	// ── 3. If a dialog is active, route all other messages to it ─────
	if m.activeDialog != noDialog {
		return m.routeToActiveDialog(msg)
	}

	// ── 4. Normal TUI handling (no dialog active) ────────────────────
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit

		case "ctrl+o":
			// Open model selector
			m.activeDialog = modelDialog
			models := m.buildModelItems()
			m.modelSelector = dialog.NewModelSelector(models)
			m.modelSelector.SetSize(m.width, m.height)
			return m, m.modelSelector.Init()

		case "ctrl+t":
			// Open theme picker
			m.activeDialog = themeDialog
			themes := m.buildThemeItems()
			m.themePicker = dialog.NewThemePicker(themes, m.currentTheme)
			return m, m.themePicker.Init()

		case "ctrl+f":
			// Open file picker
			m.activeDialog = fileDialog
			startDir := "."
			if m.app.Config != nil && m.app.Config.WorkingDir != "" {
				startDir = m.app.Config.WorkingDir
			}
			m.filePicker = dialog.NewFilePicker(startDir)
			return m, m.filePicker.Init()

		case "ctrl+b":
			// Toggle sidebar
			m.showSidebar = !m.showSidebar
			return m, nil

		case "enter":
			if m.textarea.Value() != "" && !m.loading {
				content := m.textarea.Value()
				m.textarea.Reset()

				// Check for slash commands first
				if cmd, args := m.slashRegistry.Parse(content); cmd != nil {
					output, action := cmd.Handler(args)
					
					// Add command to chat history
					m.messages = append(m.messages, ChatMessage{Role: "user", Content: content})
					
					// Execute action based on type
					switch action {
					case slash.ActionQuit:
						return m, tea.Quit
					case slash.ActionClearChat:
						m.messages = []ChatMessage{}
					case slash.ActionOpenModel:
						m.activeDialog = modelDialog
						models := m.buildModelItems()
						m.modelSelector = dialog.NewModelSelector(models)
						m.modelSelector.SetSize(m.width, m.height)
						return m, m.modelSelector.Init()
					case slash.ActionOpenTheme:
						m.activeDialog = themeDialog
						themes := m.buildThemeItems()
						m.themePicker = dialog.NewThemePicker(themes, m.currentTheme)
						return m, m.themePicker.Init()
					case slash.ActionOpenSession:
						// TODO: Implement session dialog
						m.messages = append(m.messages, ChatMessage{Role: "system", Content: "Session dialog not yet implemented"})
					case slash.ActionNewSession:
						// TODO: Implement new session
						m.messages = append(m.messages, ChatMessage{Role: "system", Content: "New session feature coming soon!"})
					case slash.ActionCompact:
						m.messages = append(m.messages, ChatMessage{Role: "system", Content: "🗜️ Session compaction coming soon!"})
					case slash.ActionShowConfig:
						if m.app.Config != nil {
							configStr := fmt.Sprintf("📋 **Current Config:**\n```\nWorkingDir: %s\nDataDir: %s\nDebug: %v\nProviders: %d configured\n```", 
								m.app.Config.WorkingDir, m.app.Config.DataDir, m.app.Config.Debug, len(m.app.Config.Providers))
							m.messages = append(m.messages, ChatMessage{Role: "system", Content: configStr})
						}
					default:
						// ActionNone or ActionOpenHelp - just show output
						if output != "" {
							m.messages = append(m.messages, ChatMessage{Role: "system", Content: output})
						}
					}
					
					m.viewport.SetContent(m.renderMessages())
					m.viewport.GotoBottom()
					return m, nil
				}

				// Regular message - send to agent
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

	// ── Streaming messages ────────────────────────────────────────
	case streamSetupMsg:
		if msg.err != nil {
			m.streaming = false
			return m, m.sendMessage(msg.content)
		}
		m.sessionID = msg.sessionID
		m.streamContentCh = msg.contentCh
		m.streamErrCh = msg.errCh
		m.startTime = time.Now() // Record start time for response tracking
		m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: ""})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, m.waitForStreamChunk()

	case streamChunkMsg:
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
		if !m.startTime.IsZero() {
			m.responseTime = time.Since(m.startTime)
			m.startTime = time.Time{}
		}
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

// routeToActiveDialog forwards a message to the currently active dialog.
func (m Model) routeToActiveDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.activeDialog {
	case modelDialog:
		var cmd tea.Cmd
		m.modelSelector, cmd = m.modelSelector.Update(msg)
		return m, cmd
	case themeDialog:
		var cmd tea.Cmd
		m.themePicker, cmd = m.themePicker.Update(msg)
		return m, cmd
	case fileDialog:
		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd
	}
	return m, nil
}

// getConfiguredProviders returns a map of providers that have API keys set.
func (m Model) getConfiguredProviders() map[string]bool {
	configured := make(map[string]bool)
	if m.app.Config != nil {
		for name, p := range m.app.Config.Providers {
			if p.APIKey != "" {
				configured[name] = true
			}
		}
	}
	return configured
}

// buildModelItems creates a list of available models from configured providers
func (m Model) buildModelItems() []dialog.ModelItem {
	var items []dialog.ModelItem
	
	// Add default models for each provider
	models := map[string][]dialog.ModelItem{
		"openai": {
			{Provider: "openai", Name: "gpt-4-turbo", ContextWindow: 128000},
			{Provider: "openai", Name: "gpt-4", ContextWindow: 8192},
			{Provider: "openai", Name: "gpt-3.5-turbo", ContextWindow: 16385},
		},
		"anthropic": {
			{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022", ContextWindow: 200000},
			{Provider: "anthropic", Name: "claude-3-opus-20240229", ContextWindow: 200000},
			{Provider: "anthropic", Name: "claude-3-haiku-20240307", ContextWindow: 200000},
		},
		"gemini": {
			{Provider: "gemini", Name: "gemini-2.0-flash", ContextWindow: 1000000},
			{Provider: "gemini", Name: "gemini-1.5-pro", ContextWindow: 2000000},
			{Provider: "gemini", Name: "gemini-1.5-flash", ContextWindow: 1000000},
		},
	}
	
	if m.app.Config != nil {
		for name, p := range m.app.Config.Providers {
			if p.APIKey != "" {
				if providerModels, ok := models[name]; ok {
					items = append(items, providerModels...)
				}
			}
		}
	}
	
	return items
}

// buildThemeItems creates a list of available themes
func (m Model) buildThemeItems() []dialog.ThemeItem {
	return []dialog.ThemeItem{
		{Name: "default", Preview: []string{"#ffffff", "#000000", "#3b82f6", "#10b981"}},
		{Name: "catppuccin", Preview: []string{"#cdd6f4", "#1e1e2e", "#cba6f7", "#a6e3a1"}},
		{Name: "dracula", Preview: []string{"#f8f8f2", "#282a36", "#bd93f9", "#50fa7b"}},
		{Name: "tokyonight", Preview: []string{"#c0caf5", "#1a1b26", "#7aa2f7", "#9ece6a"}},
	}
}

// handleModelSelection creates a new provider for the selected model and swaps it in.
func (m *Model) handleModelSelection(info dialog.ModelSelectedMsg) {
	if m.app.Agent == nil {
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: "⚠ No agent available. Configure a provider first.",
		})
		m.viewport.SetContent(m.renderMessages())
		return
	}

	// Check if the provider has an API key
	var apiKey string
	if m.app.Config != nil {
		if pCfg, ok := m.app.Config.Providers[info.Provider]; ok {
			apiKey = pCfg.APIKey
		}
	}

	if apiKey == "" {
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("⚠ No API key configured for %s. Set %s_API_KEY environment variable.", info.Provider, strings.ToUpper(info.Provider)),
		})
		m.viewport.SetContent(m.renderMessages())
		return
	}

	// Create new provider instance
	var p provider.Provider
	switch info.Provider {
	case "openai":
		p = provider.NewOpenAI(apiKey, info.Name)
	case "anthropic":
		p = provider.NewAnthropic(apiKey, info.Name)
	case "gemini":
		p = provider.NewGemini(apiKey, info.Name)
	default:
		m.messages = append(m.messages, ChatMessage{
			Role:    "system",
			Content: fmt.Sprintf("⚠ Unknown provider: %s", info.Provider),
		})
		m.viewport.SetContent(m.renderMessages())
		return
	}

	m.app.Agent.SetProvider(p)
	m.messages = append(m.messages, ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("✓ Switched to %s (%s)", info.Name, info.Provider),
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}

// formatCtxWindow formats a context window token count for display.
func formatCtxWindow(tokens int64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.0fM", float64(tokens)/1000000)
	}
	return fmt.Sprintf("%.0fK", float64(tokens)/1000)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// ── If a dialog is active, render it as an overlay ────────────
	switch m.activeDialog {
	case modelDialog:
		return m.modelSelector.View()
	case themeDialog:
		return m.themePicker.View()
	case fileDialog:
		return m.filePicker.View()
	}

	// ── Normal chat view (theme-aware) ────────────────────────────
	t := theme.GetTheme(m.currentTheme)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Padding(0, 1)

	header := titleStyle.Render("NgodeAI CLI v0.6.0")

	// ── Render status bar (bottom) ──
	var statusInfo statusbar.Info
	statusInfo.Width = m.width
	statusInfo.Streaming = m.streaming
	statusInfo.Loading = m.loading
	statusInfo.StartTime = m.startTime
	statusInfo.ResponseTime = m.responseTime
	statusInfo.PromptTokens = m.promptTokens
	statusInfo.CompletionTokens = m.completionTokens
	statusInfo.ThemeName = m.currentTheme

	if m.app.Agent != nil {
		modelInfo := m.app.Agent.Provider().Model()
		statusInfo.ModelName = modelInfo.Name
		statusInfo.ProviderName = modelInfo.Provider
		statusInfo.ContextWindow = modelInfo.ContextWindow
	}
	if m.sessionID != "" {
		statusInfo.SessionName = truncate(m.sessionID, 15)
	}

	statusBar := statusbar.Render(statusInfo, m.currentTheme)

	// ── Render sidebar (optional right panel) ──
	var sidebarView string
	sidebarWidth := 0
	if m.showSidebar && m.app.Agent != nil {
		modelInfo := m.app.Agent.Provider().Model()
		sidebarData := sidebar.Data{
			SessionName:  m.sessionID,
			MessageCount: int64(len(m.messages)),
			Tokens:       m.promptTokens + m.completionTokens,
			ModelName:    modelInfo.Name,
			ModelID:      modelInfo.ID,
			ProviderName: modelInfo.Provider,
			ContextWindow: modelInfo.ContextWindow,
			MaxTokens:    modelInfo.MaxTokens,
			ThemeName:    m.currentTheme,
		}
		sidebarView = sidebar.Render(sidebarData, m.currentTheme)
		sidebarWidth = 30 // Fixed width for now
	}

	// ── Layout: chat viewport (with optional sidebar) ──
	var chatArea string
	if m.showSidebar && sidebarView != "" {
		// Side-by-side layout
		chatWidth := m.width - sidebarWidth - 4 // Account for viewport padding
		chatStyle := lipgloss.NewStyle().Width(chatWidth)
		sidebarStyle := lipgloss.NewStyle().Width(sidebarWidth)
		
		chatArea = lipgloss.JoinHorizontal(
			lipgloss.Top,
			chatStyle.Render(m.viewport.View()),
			sidebarStyle.Render(sidebarView),
		)
	} else {
		chatArea = m.viewport.View()
	}

	footer := m.textarea.View()

	separator := strings.Repeat("─", max(m.width, 1))
	sepStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	separator = sepStyle.Render(separator)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		header,
		separator,
		chatArea,
		separator,
		footer,
		statusBar,
	)
}

func (m Model) renderMessages() string {
	t := theme.GetTheme(m.currentTheme)

	userMsgStyle := lipgloss.NewStyle().
		Foreground(t.UserMsg).
		Bold(true)

	assistantMsgStyle := lipgloss.NewStyle().
		Foreground(t.AssistantMsg).
		Bold(true)

	systemMsgStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Padding(0, 1)

	contentStyle := lipgloss.NewStyle().Padding(0, 1).MaxWidth(80)

	// Streaming cursor uses the assistant message color
	sCursor := lipgloss.NewStyle().
		Foreground(t.AssistantMsg).
		Render(" ▌")

	var b strings.Builder
	for i, msg := range m.messages {
		switch msg.Role {
		case "user":
			b.WriteString(userMsgStyle.Render("You"))
			b.WriteString("\n")
			b.WriteString(contentStyle.Render(msg.Content))
		case "assistant":
			b.WriteString(assistantMsgStyle.Render("NgodeAI"))
			b.WriteString("\n")
			content := msg.Content
			if m.streaming && i == len(m.messages)-1 {
				content += sCursor
			}
			b.WriteString(contentStyle.Render(content))
		case "system":
			b.WriteString(systemMsgStyle.Render(msg.Content))
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "…"
}

// ── Streaming tea.Msg types ──────────────────────────────────────────────────

// streamSetupMsg is sent once the streaming goroutine has created the session
// and obtained the content/error channels from the agent.
type streamSetupMsg struct {
	contentCh <-chan string
	errCh     <-chan error
	sessionID string
	content   string
	err       error
}

// streamChunkMsg carries a single content delta from the streaming response.
type streamChunkMsg string

// streamDoneMsg signals that the stream has finished.
type streamDoneMsg struct {
	err error
}

// ── Streaming commands ───────────────────────────────────────────────────────

func (m Model) startStreaming(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		sessionID := m.sessionID
		if sessionID == "" {
			sess, err := m.app.Sessions.Create("TUI Session")
			if err != nil {
				return streamSetupMsg{content: content, err: err}
			}
			sessionID = sess.ID
		}

		contentCh, errCh := m.app.Agent.StreamRun(ctx, sessionID, content)
		return streamSetupMsg{
			contentCh: contentCh,
			errCh:     errCh,
			sessionID: sessionID,
			content:   content,
		}
	}
}

func (m Model) waitForStreamChunk() tea.Cmd {
	contentCh := m.streamContentCh
	errCh := m.streamErrCh
	if contentCh == nil && errCh == nil {
		return nil
	}

	return func() tea.Msg {
		if contentCh != nil {
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
			default:
			}
		}

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

type responseMsg struct {
	content   string
	sessionID string
	err       error
}

func (m Model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		sessionID := m.sessionID
		if sessionID == "" {
			sess, err := m.app.Sessions.Create("TUI Session")
			if err != nil {
				return responseMsg{err: err}
			}
			sessionID = sess.ID
		}

		response, err := m.app.Agent.Run(ctx, sessionID, content)
		if err != nil {
			return responseMsg{err: err, sessionID: sessionID}
		}

		return responseMsg{content: response, sessionID: sessionID}
	}
}
