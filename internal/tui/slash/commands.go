package slash

import (
	"fmt"
	"strings"
)

// Command represents a slash command
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Shortcut    string // Keyboard shortcut hint (e.g., "ctrl+o")
	Args        string // Optional args description (e.g., "<model-name>")
	Handler     func(args string) (string, Action)
}

// Action defines what should happen after a slash command executes
type Action int

const (
	ActionNone       Action = iota // Just show output in chat
	ActionQuit                     // Quit the app
	ActionClearChat                // Clear chat messages
	ActionOpenModel                // Open model selector dialog
	ActionOpenTheme                // Open theme picker dialog
	ActionOpenHelp                 // Open help overlay
	ActionOpenSession              // Open session dialog
	ActionCompact                  // Trigger session compaction
	ActionNewSession               // Create new session
	ActionShowConfig               // Show current config
)

// Registry holds all registered slash commands
type Registry struct {
	commands []Command
}

// NewRegistry creates a new command registry with default commands
func NewRegistry() *Registry {
	r := &Registry{}
	r.registerDefaults()
	return r
}

// Parse parses input text and returns a command if it starts with /
func (r *Registry) Parse(input string) (*Command, string) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return nil, ""
	}

	// Remove the leading /
	input = input[1:]

	// Split into command name and args
	parts := strings.SplitN(input, " ", 2)
	name := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	// Find matching command
	for _, cmd := range r.commands {
		if strings.EqualFold(cmd.Name, name) {
			return &cmd, args
		}
		for _, alias := range cmd.Aliases {
			if strings.EqualFold(alias, name) {
				return &cmd, args
			}
		}
	}

	return nil, ""
}

// List returns all registered commands
func (r *Registry) List() []Command {
	return r.commands
}

// registerDefaults adds all built-in commands
func (r *Registry) registerDefaults() {
	r.commands = []Command{
		{
			Name:        "help",
			Aliases:     []string{"h", "?"},
			Description: "Show available commands and shortcuts",
			Shortcut:    "ctrl+h",
			Handler: func(args string) (string, Action) {
				var sb strings.Builder
				sb.WriteString("📖 **Available Commands:**\n\n")
				sb.WriteString("```\n")
				sb.WriteString(fmt.Sprintf("  %-15s %-30s %s\n", "COMMAND", "DESCRIPTION", "SHORTCUT"))
				sb.WriteString(fmt.Sprintf("  %-15s %-30s %s\n", "───────", "───────────", "────────"))
				for _, cmd := range r.commands {
					shortcut := cmd.Shortcut
					if shortcut == "" {
						shortcut = "—"
					}
					name := "/" + cmd.Name
					sb.WriteString(fmt.Sprintf("  %-15s %-30s %s\n", name, cmd.Description, shortcut))
				}
				sb.WriteString("```\n")
				sb.WriteString("\n💡 Type `/` and press Tab to see autocomplete suggestions.")
				return sb.String(), ActionOpenHelp
			},
		},
		{
			Name:        "model",
			Aliases:     []string{"m"},
			Description: "Change AI model/provider",
			Shortcut:    "ctrl+o",
			Handler: func(args string) (string, Action) {
				if args == "" {
					return "", ActionOpenModel
				}
				return fmt.Sprintf("🔄 Switching model to: %s", args), ActionNone
			},
		},
		{
			Name:        "theme",
			Aliases:     []string{"t"},
			Description: "Change color theme",
			Shortcut:    "ctrl+t",
			Handler: func(args string) (string, Action) {
				return "", ActionOpenTheme
			},
		},
		{
			Name:        "session",
			Aliases:     []string{"s"},
			Description: "Switch or list sessions",
			Shortcut:    "ctrl+s",
			Handler: func(args string) (string, Action) {
				return "", ActionOpenSession
			},
		},
		{
			Name:        "new",
			Aliases:     []string{"n"},
			Description: "Start a new session",
			Shortcut:    "ctrl+n",
			Handler: func(args string) (string, Action) {
				return "", ActionNewSession
			},
		},
		{
			Name:        "compact",
			Aliases:     []string{"c"},
			Description: "Compact session history (summarize)",
			Handler: func(args string) (string, Action) {
				return "🗜️ Compacting session history...", ActionCompact
			},
		},
		{
			Name:        "clear",
			Aliases:     []string{"cls"},
			Description: "Clear the chat screen",
			Handler: func(args string) (string, Action) {
				return "", ActionClearChat
			},
		},
		{
			Name:        "tokens",
			Aliases:     []string{"usage", "stats"},
			Description: "Show token usage and stats",
			Handler: func(args string) (string, Action) {
				return "📊 **Token Usage**\n\n```\nSession tokens: Calculating...\nTotal cost: $0.00\n```", ActionNone
			},
		},
		{
			Name:        "config",
			Aliases:     []string{"cfg"},
			Description: "Show current configuration",
			Handler: func(args string) (string, Action) {
				return "", ActionShowConfig
			},
		},
		{
			Name:        "skills",
			Aliases:     []string{"sk"},
			Description: "List available skills",
			Handler: func(args string) (string, Action) {
				var sb strings.Builder
				sb.WriteString("🧠 **Available Skills**\n\n")
				sb.WriteString("No custom skills loaded.\n")
				sb.WriteString("\n💡 Skills are markdown-based prompts stored in:\n")
				sb.WriteString("  • `~/.ngodeai/skills/*.md` (user skills)\n")
				sb.WriteString("  • `.ngodeai/skills/*.md` (project skills)\n")
				return sb.String(), ActionNone
			},
		},
		{
			Name:        "quit",
			Aliases:     []string{"q", "exit"},
			Description: "Quit NgodeAI",
			Shortcut:    "ctrl+c",
			Handler: func(args string) (string, Action) {
				return "", ActionQuit
			},
		},
	}
}
