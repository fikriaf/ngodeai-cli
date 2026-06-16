package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Command represents a custom command loaded from a markdown file
type Command struct {
	Name        string // Command name (derived from filename without .md)
	Description string // First line of the file (used as description)
	Template    string // Full content as the prompt template
}

// Service manages custom commands
type Service struct {
	commandsDir string
	commands    map[string]Command
}

// NewService creates a new commands service
func NewService(workingDir string) *Service {
	return &Service{
		commandsDir: filepath.Join(workingDir, ".ngode", "commands"),
		commands:    make(map[string]Command),
	}
}

// Load reads all command files from the .ngode/commands/ directory
func (s *Service) Load() error {
	// Check if commands directory exists
	if _, err := os.Stat(s.commandsDir); os.IsNotExist(err) {
		// No commands directory is fine - just means no custom commands
		return nil
	}

	// Read all .md files in the commands directory
	entries, err := os.ReadDir(s.commandsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .md files
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		// Read the file content
		filePath := filepath.Join(s.commandsDir, name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Extract command name (filename without .md extension)
		cmdName := strings.TrimSuffix(name, filepath.Ext(name))

		// Extract description (first non-empty line)
		lines := strings.Split(string(content), "\n")
		description := ""
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				// Remove markdown heading markers if present
				description = strings.TrimLeft(trimmed, "# ")
				break
			}
		}

		// Store the command
		s.commands[cmdName] = Command{
			Name:        cmdName,
			Description: description,
			Template:    string(content),
		}
	}

	return nil
}

// List returns all available commands
func (s *Service) List() []Command {
	result := make([]Command, 0, len(s.commands))
	for _, cmd := range s.commands {
		result = append(result, cmd)
	}
	return result
}

// Get returns a command by name
func (s *Service) Get(name string) (Command, bool) {
	cmd, ok := s.commands[name]
	return cmd, ok
}

// Help generates a help message for available commands
func (s *Service) Help() string {
	if len(s.commands) == 0 {
		return "No custom commands available. Add .md files to ~/.ngodeai/.ngode/commands/ or $(pwd)/.ngode/commands/"
	}

	var sb strings.Builder
	sb.WriteString("Available Custom Commands:\n\n")

	for _, cmd := range s.List() {
		sb.WriteString(fmt.Sprintf("/%s - %s\n", cmd.Name, cmd.Description))
	}

	return sb.String()
}

// Execute renders the command template with the given variables
func (s *Service) Execute(name string, vars map[string]string) (string, bool) {
	cmd, ok := s.commands[name]
	if !ok {
		return "", false
	}

	// Perform variable substitution
	result := cmd.Template
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Also handle common aliases
	if input, ok := vars["input"]; ok {
		result = strings.ReplaceAll(result, "{{query}}", input)
		result = strings.ReplaceAll(result, "{{text}}", input)
	}

	return result, true
}

// HasCommands returns true if there are any loaded commands
func (s *Service) HasCommands() bool {
	return len(s.commands) > 0
}

// FormatList formats commands for display in a compact list
func (s *Service) FormatList() []string {
	result := make([]string, 0, len(s.commands))
	for _, cmd := range s.List() {
		result = append(result, fmt.Sprintf("/%s - %s", cmd.Name, cmd.Description))
	}
	return result
}

