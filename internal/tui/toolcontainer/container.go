package toolcontainer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fikriaf/ngodeai-cli/internal/tui/theme"
)

// ToolExecution represents a tool execution with metadata
type ToolExecution struct {
	Name       string
	Parameters map[string]interface{}
	Output     string
	Error      bool
	Duration   time.Duration
	Timestamp  time.Time
}

// Render creates a visual container for tool execution
func Render(exec ToolExecution, themeName string, maxWidth int) string {
	t := theme.GetTheme(themeName)

	// Container styling
	borderColor := t.Success
	if exec.Error {
		borderColor = t.Error
	}

	// Header with tool name and status
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Background).
		Background(borderColor).
		Padding(0, 1)

	statusIcon := "✓"
	statusText := "Success"
	if exec.Error {
		statusIcon = "✗"
		statusText = "Failed"
	}

	durationStr := ""
	if exec.Duration > 0 {
		durationStr = fmt.Sprintf(" (%.1fs)", exec.Duration.Seconds())
	}

	header := headerStyle.Render(fmt.Sprintf("TOOL: %s", exec.Name))
	status := lipgloss.NewStyle().
		Foreground(borderColor).
		Bold(true).
		Render(fmt.Sprintf("%s %s%s", statusIcon, statusText, durationStr))

	// Parameters section
	var paramsSection string
	if len(exec.Parameters) > 0 {
		paramsStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Padding(0, 1)

		var paramsBuilder strings.Builder
		paramsBuilder.WriteString("Parameters:\n")
		for key, value := range exec.Parameters {
			valueStr := formatParameterValue(value)
			paramsBuilder.WriteString(fmt.Sprintf("  %s: %s\n", key, valueStr))
		}
		paramsSection = paramsStyle.Render(paramsBuilder.String())
	}

	// Output section
	outputStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Padding(0, 1)

	outputSection := ""
	if exec.Output != "" {
		// Truncate very long outputs
		output := exec.Output
		if len(output) > 500 {
			output = output[:500] + "\n... (truncated)"
		}
		outputSection = outputStyle.Render(output)
	}

	// Build the container
	var container strings.Builder

	// Top border
	container.WriteString(lipgloss.NewStyle().
		Foreground(borderColor).
		Render(strings.Repeat("─", maxWidth-2)))
	container.WriteString("\n")

	// Header line
	container.WriteString(header)
	container.WriteString(" ")
	container.WriteString(status)
	container.WriteString("\n")

	// Separator
	container.WriteString(lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(strings.Repeat("─", maxWidth-2)))
	container.WriteString("\n")

	// Parameters (if any)
	if paramsSection != "" {
		container.WriteString(paramsSection)
		container.WriteString("\n")
	}

	// Output
	if outputSection != "" {
		container.WriteString(outputSection)
		container.WriteString("\n")
	}

	// Bottom border
	container.WriteString(lipgloss.NewStyle().
		Foreground(borderColor).
		Render(strings.Repeat("─", maxWidth-2)))

	return container.String()
}

// formatParameterValue formats parameter values for display
func formatParameterValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Truncate long strings
		if len(v) > 60 {
			return fmt.Sprintf("%q...", v[:60])
		}
		return fmt.Sprintf("%q", v)
	case []string:
		if len(v) > 3 {
			return fmt.Sprintf("[%q, ... (%d items)]", v[0], len(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RenderInline creates a compact inline version for streaming
func RenderInline(toolName string, isRunning bool, themeName string) string {
	t := theme.GetTheme(themeName)

	icon := "⚡"
	color := t.Warning
	if !isRunning {
		icon = "✓"
		color = t.Success
	}

	style := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	return style.Render(fmt.Sprintf("%s %s", icon, toolName))
}
