package sidebar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/tui/theme"
)

// Data holds all data needed for the sidebar
type Data struct {
	// Session info
	SessionName string
	MessageCount int64
	Tokens       int64
	CostUSD      float64

	// Model info
	ModelName    string
	ModelID      string
	ProviderName string
	ContextWindow int64
	MaxTokens    int64

	// MCP servers
	MCPs []MCPInfo

	// Theme
	ThemeName string
}

// MCPInfo represents an MCP server connection
type MCPInfo struct {
	Name   string
	Status Status // Connected, Disconnected, Connecting, Error
}

// Status represents MCP connection status
type Status int

const (
	StatusConnected Status = iota
	StatusDisconnected
	StatusConnecting
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusConnected:
		return "✅ Connected"
	case StatusDisconnected:
		return "❌ Disconnected"
	case StatusConnecting:
		return "⏳ Connecting..."
	case StatusError:
		return "⚠️ Error"
	default:
		return "Unknown"
	}
}

// Render produces the sidebar HTML-like string
func Render(data Data, themeName string) string {
	t := theme.GetTheme(themeName)

	var b strings.Builder

	// ── Header ──
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Padding(0, 1)
	
	b.WriteString(headerStyle.Render("╭───────────────────╮") + "\n")
	b.WriteString(headerStyle.Render("│ 🔧 NgodeAI CLI │") + "\n")
	b.WriteString(headerStyle.Render("╰───────────────────╯") + "\n")
	b.WriteString("\n")

	// ── Session Section ──
	if data.SessionName != "" || data.MessageCount > 0 {
		sectionTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Text).
			Render("📋 Session")
		sectionBody := fmt.Sprintf("  Name: %s\n  Messages: %d\n", 
			truncate(data.SessionName, 20), 
			data.MessageCount)
		
		if data.CostUSD > 0 {
			sectionBody += fmt.Sprintf("  Cost: $%.2f\n", data.CostUSD)
		}
		
		b.WriteString(sectionTitle + "\n")
		for _, line := range strings.Split(sectionBody, "\n") {
			b.WriteString(lipgloss.NewStyle().Foreground(t.TextMuted).Render(line) + "\n")
		}
		b.WriteString("\n")
	}

	// ── Model Section ──
	sectionTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Text).
		Render("🧠 Model")
	sectionBody := fmt.Sprintf("  Model: %s\n  Provider: %s\n", 
		truncate(data.ModelName, 20), 
		truncate(data.ProviderName, 15))
	
	if data.MaxTokens > 0 && data.ContextWindow > 0 {
		pct := float64(data.Tokens) / float64(data.ContextWindow) * 100
		barWidth := 20
		filled := int(float64(pct) / 100.0 * float64(barWidth))
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}
		
		progressBar := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF6600")).
			Render(strings.Repeat("█", filled))
		
		emptyBar := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(strings.Repeat("░", barWidth-filled))
		
		sectionBody += fmt.Sprintf("  Context: %.0fK/%dK (%s%s)\n", 
			float64(data.Tokens)/1000,
			float64(data.ContextWindow)/1000,
			progressBar,
			emptyBar)
		sectionBody += fmt.Sprintf("  Max output: %d tokens\n", data.MaxTokens)
	}
	
	b.WriteString(sectionTitle + "\n")
	for _, line := range strings.Split(sectionBody, "\n") {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextMuted).Render(line) + "\n")
	}
	b.WriteString("\n")

	// ── MCP Servers Section ──
	sectionTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Text).
		Render("🔌 MCP Servers")
	
	if len(data.MCPs) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextMuted).Render("  No MCP servers configured") + "\n\n")
	} else {
		for _, mcp := range data.MCPs {
			statusText := mcp.Status.String()
			
			if mcp.Status == StatusError {
				statusText = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FF6600")).
					Render(mcp.Status.String())
			}
			
			b.WriteString(fmt.Sprintf("  • %s %s\n", mcp.Name, statusText))
		}
	}

	// ── Footer ──
	footer := lipgloss.NewStyle().
		Faint(true).
		Foreground(t.TextMuted).
		Render("  Tips: Type '/' for commands · Ctrl+C to quit")

	b.WriteString(footer + "\n")
	b.WriteString("  " + strings.Repeat("─", 19) + "\n")

	return b.String()
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
