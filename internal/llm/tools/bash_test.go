package tools

import (
	"context"
	"strings"
	"testing"
)

func TestBashTool_BannedCommands(t *testing.T) {
	bash := NewBashTool()

	tests := []struct {
		name    string
		command string
		blocked bool
	}{
		{"rm -rf root", "rm -rf /", true},
		{"mkfs", "mkfs.ext4 /dev/sda1", true},
		{"fork bomb", ":(){:|:&};:", true},
		{"dd zero", "dd if=/dev/zero of=/dev/sda", true},
		{"ls safe", "echo hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := bash.Run(ctx, ToolCall{
				Name: "bash",
				Args: map[string]any{"command": tt.command},
			})

			if err != nil {
				t.Fatalf("Unexpected Go error: %v", err)
			}

			if tt.blocked {
				if !result.IsError {
					t.Errorf("Expected command to be blocked: %s", tt.command)
				}
				if !strings.Contains(result.Content, "blocked") {
					t.Errorf("Expected 'blocked' in error message, got: %s", result.Content)
				}
			} else {
				if result.IsError {
					t.Errorf("Expected command to succeed: %s, got: %s", tt.command, result.Content)
				}
			}
		})
	}
}

func TestBashTool_MissingCommand(t *testing.T) {
	bash := NewBashTool()
	ctx := context.Background()

	result, err := bash.Run(ctx, ToolCall{
		Name: "bash",
		Args: map[string]any{},
	})

	if err != nil {
		t.Fatalf("Unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for missing command parameter")
	}
}

func TestBashTool_SuccessfulCommand(t *testing.T) {
	bash := NewBashTool()
	ctx := context.Background()

	result, err := bash.Run(ctx, ToolCall{
		Name: "bash",
		Args: map[string]any{"command": "echo 'hello world'"},
	})

	if err != nil {
		t.Fatalf("Unexpected Go error: %v", err)
	}
	if result.IsError {
		t.Errorf("Expected success, got error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "hello world") {
		t.Errorf("Expected 'hello world' in output, got: %s", result.Content)
	}
}
