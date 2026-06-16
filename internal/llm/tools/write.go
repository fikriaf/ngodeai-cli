package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool creates or overwrites a file
type WriteTool struct{}

func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

func (w *WriteTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "write",
		Description: "Create or overwrite a file with the given content. Parent directories are created automatically.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "File content to write",
				},
			},
			"required": []string{"path", "content"},
		},
		Required: []string{"path", "content"},
	}
}

func (w *WriteTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	path, _ := params.Args["path"].(string)
	content, _ := params.Args["content"].(string)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Failed to create directory: %v", err)}, nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Failed to write file: %v", err)}, nil
	}

	return ToolResponse{
		Content: fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path),
		Path:    path,
	}, nil
}
