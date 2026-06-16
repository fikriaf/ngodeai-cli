package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool finds files matching patterns
type GlobTool struct{}

func NewGlobTool() *GlobTool {
	return &GlobTool{}
}

func (g *GlobTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "glob",
		Description: "Find files matching a glob pattern. Returns file paths sorted by modification time.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern (e.g., '*.go', '**/*.ts')",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Base directory (default: current directory)",
				},
			},
			"required": []string{"pattern"},
		},
		Required: []string{"pattern"},
	}
}

func (g *GlobTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	pattern, _ := params.Args["pattern"].(string)
	basePath, _ := params.Args["path"].(string)

	if basePath == "" {
		basePath = "."
	}

	fullPattern := filepath.Join(basePath, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return ToolResponse{IsError: true, Content: fmt.Sprintf("Invalid pattern: %v", err)}, nil
	}

	if len(matches) == 0 {
		return ToolResponse{Content: "No files found matching pattern"}, nil
	}

	sort.Strings(matches)

	result := strings.Join(matches, "\n")
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}

	return ToolResponse{
		Content: fmt.Sprintf("Found %d files:\n%s", len(matches), result),
	}, nil
}
