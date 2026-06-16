package tools

import (
	"context"
)

// BaseTool is the interface all tools must implement
type BaseTool interface {
	// Info returns tool metadata for LLM function calling
	Info() ToolInfo

	// Run executes the tool with given parameters
	Run(ctx context.Context, params ToolCall) (ToolResponse, error)
}

// ToolInfo describes a tool for the LLM
type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
}

// ToolCall represents a tool invocation request
type ToolCall struct {
	CallID string
	Name   string
	Args   map[string]any
}

// ToolResponse is the result of a tool execution
type ToolResponse struct {
	IsError bool
	Content string
	Path    string
	Version string
}
