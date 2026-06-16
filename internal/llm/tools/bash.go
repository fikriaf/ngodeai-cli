package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes shell commands
type BashTool struct{}

func NewBashTool() *BashTool {
	return &BashTool{}
}

func (b *BashTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "bash",
		Description: "Execute a shell command. Use for running scripts, installing packages, git operations, etc.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		Required: []string{"command"},
	}
}

// banned commands for security
var bannedCommands = []string{
	"rm -rf /",
	"mkfs",
	"dd if=/dev/zero",
	":(){:|:&};:",
}

func (b *BashTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	command, ok := params.Args["command"].(string)
	if !ok {
		return ToolResponse{IsError: true, Content: "command parameter required"}, nil
	}

	// Check banned commands
	for _, banned := range bannedCommands {
		if strings.Contains(command, banned) {
			return ToolResponse{
				IsError: true,
				Content: fmt.Sprintf("Command blocked for security: %s", banned),
			}, nil
		}
	}

	// Execute with timeout
	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return ToolResponse{
			IsError: true,
			Content: fmt.Sprintf("Command failed: %v\nOutput:\n%s", err, string(output)),
		}, nil
	}

	// Truncate long output
	result := string(output)
	if len(result) > 10000 {
		result = result[:10000] + "\n... (truncated)"
	}

	return ToolResponse{
		Content: result,
	}, nil
}
