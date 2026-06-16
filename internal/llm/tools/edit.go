package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// EditTool performs search-and-replace edits in files
type EditTool struct{}

func NewEditTool() *EditTool {
	return &EditTool{}
}

func (e *EditTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "edit",
		Description: "Edit a file by replacing an exact text match with new text. The old_string must be unique in the file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path to edit",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact text to find (must be unique)",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Text to replace with",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Required: []string{"path", "old_string", "new_string"},
	}
}

func (e *EditTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	path, _ := params.Args["path"].(string)
	oldStr, _ := params.Args["old_string"].(string)
	newStr, _ := params.Args["new_string"].(string)

	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Failed to read file: %v", err)}, nil
	}

	original := string(content)

	// Check uniqueness
	count := strings.Count(original, oldStr)
	if count == 0 {
		return ToolResponse{IsError: true, Content: "old_string not found in file"}, nil
	}
	if count > 1 {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("old_string found %d times, must be unique", count)}, nil
	}

	// Replace
	updated := strings.Replace(original, oldStr, newStr, 1)

	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Failed to write file: %v", err)}, nil
	}

	return ToolResponse{
		Content: fmt.Sprintf("Successfully edited %s", path),
		Path:    path,
	}, nil
}
