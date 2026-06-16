package tools

import (
	"context"
	"fmt"
	"os"
)

// ViewTool reads file contents
type ViewTool struct{}

func NewViewTool() *ViewTool {
	return &ViewTool{}
}

func (v *ViewTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "view",
		Description: "Read the contents of a file",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The file path to read",
				},
			},
			"required": []string{"path"},
		},
		Required: []string{"path"},
	}
}

func (v *ViewTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	path, ok := params.Args["path"].(string)
	if !ok {
		return ToolResponse{IsError: true, Content: "path parameter required"}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Failed to read file: %v", err)}, nil
	}

	return ToolResponse{
		Content: string(content),
		Path:    path,
	}, nil
}
