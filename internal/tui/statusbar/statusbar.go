package statusbar

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/tui/theme"
)

// Info holds all data needed to render the status bar
type Info struct {
	// Model info
	ModelName    string
	ProviderName string
	ModelColor   string // hex color for the model indicator dot

	// Token usage
	PromptTokens     int64
	CompletionTokens int64
	ContextWindow    int64

	// Timing
	ResponseTime time.Duration
	Streaming    bool
	Loading      bool
	StartTime    time.Time // when streaming/thinking started

	// Session
	SessionName string
	MessageCount int

	// Theme
	ThemeName string

	// MCP
	MCPConnected int
	MCPTotal     int

	// Cost (optional)
	CostUSD float64

	// Width of the terminal
	Width int
}

// Render produces the status bar string
func Render(info Info, themeName string) string {
	t := theme.GetTheme(themeName)

	// ── Left section: model indicator + name ──
	modelDot := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Render("●")

	modelName := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render(truncate(info.ModelName, 25))

	providerLabel := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(fmt.Sprintf(" (%s)", info.ProviderName))

	left := fmt.Sprintf("%s %s%s", modelDot, modelName, providerLabel)

	// ── Center section: status / thinking / streaming ──
	var center string
	if info.Streaming || info.Loading {
		elapsed := time.Since(info.StartTime).Truncate(time.Second)
		if info.Streaming {
			center = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFAA00")).
				Render(fmt.Sprintf("⚡ Streaming… %s", elapsed))
		} else {
			center = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6600")).
				Render(fmt.Sprintf("🧠 Thinking… %s", elapsed))
		}
	} else if info.ResponseTime > 0 {
		center = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("⏱️ %s", info.ResponseTime.Truncate(time.Millisecond*100)))
	}

	// ── Right section: tokens + session + keybinds ──
	totalTokens := info.PromptTokens + info.CompletionTokens
	tokenStr := formatTokens(totalTokens)

	var contextPct string
	if info.ContextWindow > 0 {
		pct := float64(totalTokens) / float64(info.ContextWindow) * 100
		contextPct = fmt.Sprintf("%.0f%%", pct)
	}

	tokensSection := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(fmt.Sprintf("📊 %s", tokenStr))

	if contextPct != "" {
		tokensSection = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("📊 %s (%s)", tokenStr, contextPct))
	}

	sessionSection := ""
	if info.SessionName != "" {
		sessionSection = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("💬 %s", truncate(info.SessionName, 15)))
	}

	mcpSection := ""
	if info.MCPTotal > 0 {
		mcpSection = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("🔌 %d/%d", info.MCPConnected, info.MCPTotal))
	}

	costSection := ""
	if info.CostUSD > 0 {
		costSection = lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(fmt.Sprintf("💰 %s", formatCost(info.CostUSD)))
	}

	keybindHint := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Faint(true).
		Render("/help")

	// Assemble right side
	rightParts := []string{}
	if tokensSection != "" {
		rightParts = append(rightParts, tokensSection)
	}
	if costSection != "" {
		rightParts = append(rightParts, costSection)
	}
	if mcpSection != "" {
		rightParts = append(rightParts, mcpSection)
	}
	if sessionSection != "" {
		rightParts = append(rightParts, sessionSection)
	}
	rightParts = append(rightParts, keybindHint)
	right := strings.Join(rightParts, " · ")

	// ── Layout: left | center | right ──
	width := info.Width
	if width <= 0 {
		width = 80
	}

	// Build the bar with padding
	separator := " │ "

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	centerWidth := lipgloss.Width(center)

	// Calculate spacing
	available := width - leftWidth - rightWidth - lipgloss.Width(separator)*2
	if available < 0 {
		available = 0
	}

	var bar string
	if centerWidth > 0 && available > centerWidth+4 {
		// Center the middle section
		leftPad := (available - centerWidth) / 2
		rightPad := available - centerWidth - leftPad
		bar = left + separator + strings.Repeat(" ", leftPad) + center + strings.Repeat(" ", rightPad) + separator + right
	} else {
		// No center, just left and right
		gap := width - leftWidth - rightWidth - lipgloss.Width(separator)
		if gap < 1 {
			gap = 1
		}
		bar = left + separator + strings.Repeat(" ", gap) + right
	}

	// Style the full bar
	bgColor := t.Background
	if bgColor == "" {
		bgColor = "#1a1a2e"
	}

	barStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(bgColor)).
		Width(width).
		Padding(0, 1)

	return barStyle.Render(bar)
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

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(cost float64) string {
	if cost < 0.01 {
		return "< $0.01"
	}
	if cost < 1.00 {
		cents := cost * 100
		return fmt.Sprintf("%.2f¢", cents)
	}
	return fmt.Sprintf("$%.2f", cost)
}
