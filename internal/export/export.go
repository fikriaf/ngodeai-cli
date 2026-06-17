package export

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ChatMessage represents a chat message for export
type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model,omitempty"`
}

// ChatExport represents the full chat export
type ChatExport struct {
	SessionID string        `json:"session_id"`
	Exported  time.Time     `json:"exported_at"`
	Messages  []ChatMessage `json:"messages"`
}

// ExportMarkdown exports chat to markdown format
func ExportMarkdown(messages []ChatMessage, sessionID string) string {
	var md string
	md += fmt.Sprintf("# NgodeAI Chat Export\n\n")
	md += fmt.Sprintf("**Session ID:** %s\n", sessionID)
	md += fmt.Sprintf("**Exported:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	md += "---\n\n"

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			md += fmt.Sprintf("## 👤 User (%s)\n\n", msg.Timestamp.Format("15:04:05"))
			md += msg.Content + "\n\n"
		case "assistant":
			md += fmt.Sprintf("## 🤖 Assistant (%s)\n", msg.Timestamp.Format("15:04:05"))
			if msg.Model != "" {
				md += fmt.Sprintf("*Model: %s*\n", msg.Model)
			}
			md += "\n" + msg.Content + "\n\n"
		case "system":
			md += fmt.Sprintf("## ⚙️ System (%s)\n\n", msg.Timestamp.Format("15:04:05"))
			md += msg.Content + "\n\n"
		}
		md += "---\n\n"
	}

	return md
}

// ExportJSON exports chat to JSON format
func ExportJSON(messages []ChatMessage, sessionID string) (string, error) {
	export := ChatExport{
		SessionID: sessionID,
		Exported:  time.Now(),
		Messages:  messages,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

// SaveToFile saves content to a file
func SaveToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}
