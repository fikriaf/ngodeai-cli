package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GrepTool searches file contents using ripgrep
type GrepTool struct{}

func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

func (g *GrepTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "grep",
		Description: "Search file contents using regex patterns. Uses ripgrep if available, falls back to grep.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Regex pattern to search for",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in (default: current directory)",
				},
				"include": map[string]any{
					"type":        "string",
					"description": "File pattern to include (e.g., '*.go')",
				},
			},
			"required": []string{"pattern"},
		},
		Required: []string{"pattern"},
	}
}

func (g *GrepTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	pattern, _ := params.Args["pattern"].(string)
	path, _ := params.Args["path"].(string)
	include, _ := params.Args["include"].(string)

	if path == "" {
		path = "."
	}

	// Try ripgrep first
	args := []string{"-n", "--no-heading", pattern, path}
	if include != "" {
		args = append([]string{"-g", include}, args...)
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Fall back to grep
		grepArgs := []string{"-rn", pattern, path}
		if include != "" {
			grepArgs = append(grepArgs, "--include="+include)
		}
		cmd = exec.CommandContext(ctx, "grep", grepArgs...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			// No matches is exit code 1 for grep
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return ToolResponse{Content: "No matches found"}, nil
			}
			return ToolResponse{IsError: true, Content: fmt.Sprintf("Search failed: %v", err)}, nil
		}
	}

	result := string(output)
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}

	lines := strings.Count(result, "\n")
	return ToolResponse{
		Content: fmt.Sprintf("Found %d matches:\n%s", lines, result),
	}, nil
}
