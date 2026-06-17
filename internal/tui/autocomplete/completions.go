package autocomplete

import (
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/tui/slash"
)

// Completion represents a completion suggestion
type Completion struct {
	Text        string
	Description string
	CursorPos   int // Where to place cursor after completion
}

// GetCompletions returns completion suggestions for the current input
func GetCompletions(input string, registry *slash.Registry, cursorPos int) []Completion {
	// Only provide completions at the start of input or after whitespace
	if cursorPos == 0 {
		return nil
	}

	// Get text up to cursor
	textBeforeCursor := input[:cursorPos]
	
	// Check if we're completing a slash command
	if strings.HasPrefix(textBeforeCursor, "/") {
		return getCommandCompletions(textBeforeCursor, registry)
	}

	return nil
}

// getCommandCompletions returns completions for slash commands
func getCommandCompletions(input string, registry *slash.Registry) []Completion {
	// Remove the leading slash
	prefix := strings.TrimPrefix(input, "/")
	
	// Split into command and args
	parts := strings.SplitN(prefix, " ", 2)
	commandPart := parts[0]
	
	// If there's a space, we're completing arguments, not the command itself
	if len(parts) > 1 {
		return getArgumentCompletions(commandPart, parts[1], registry)
	}
	
	// Get all commands
	commands := registry.List()
	var completions []Completion
	
	// Filter commands by prefix
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Name, commandPart) {
			completions = append(completions, Completion{
				Text:        "/" + cmd.Name,
				Description: cmd.Description,
				CursorPos:   len(cmd.Name) + 1, // +1 for the slash
			})
		}
		// Also check aliases
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(alias, commandPart) {
				completions = append(completions, Completion{
					Text:        "/" + alias,
					Description: cmd.Description + " (alias)",
					CursorPos:   len(alias) + 1,
				})
			}
		}
	}
	
	return completions
}

// getArgumentCompletions returns completions for command arguments
func getArgumentCompletions(command, argPrefix string, registry *slash.Registry) []Completion {
	// Find the command
	cmd, _ := registry.Parse("/" + command)
	if cmd == nil {
		return nil
	}
	
	var completions []Completion
	
	// Command-specific argument completions
	switch command {
	case "model":
		// Could provide model name completions here
		// For now, just suggest common models
		models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opus", "claude-3-sonnet"}
		for _, model := range models {
			if strings.HasPrefix(model, argPrefix) {
				completions = append(completions, Completion{
					Text:        model,
					Description: "Model name",
					CursorPos:   len("/"+command+" "+model),
				})
			}
		}
		
	case "theme":
		// Theme name completions
		themes := []string{"default", "dracula", "catppuccin", "tokyonight", "gruvbox", "monokai", "onedark"}
		for _, theme := range themes {
			if strings.HasPrefix(theme, argPrefix) {
				completions = append(completions, Completion{
					Text:        theme,
					Description: "Theme name",
					CursorPos:   len("/"+command+" "+theme),
				})
			}
		}
	}
	
	return completions
}

// ApplyCompletion applies a completion to the input
func ApplyCompletion(input string, completion Completion, cursorPos int) (string, int) {
	// Find the start of the word being completed
	wordStart := cursorPos
	for wordStart > 0 && input[wordStart-1] != ' ' {
		wordStart--
	}
	
	// Replace the word with the completion
	newInput := input[:wordStart] + completion.Text
	if cursorPos < len(input) {
		newInput += input[cursorPos:]
	}
	
	// Add a space after the completion if it's a command
	if strings.HasPrefix(completion.Text, "/") && !strings.Contains(completion.Text, " ") {
		newInput += " "
		completion.CursorPos++
	}
	
	return newInput, wordStart + completion.CursorPos
}
