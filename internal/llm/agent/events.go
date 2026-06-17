package agent

import (
	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/message"
)

// EventType identifies different agent events
type EventType string

const (
	EventContentDelta EventType = "content_delta"
	EventToolStart    EventType = "tool_start"
	EventToolEnd      EventType = "tool_end"
	EventToolResult   EventType = "tool_result"
	EventTokens       EventType = "tokens"
	EventDone         EventType = "done"
	EventError        EventType = "error"
	EventIteration    EventType = "iteration"
)

// AgentEvent represents a rich event from the agent loop
type AgentEvent struct {
	Type EventType

	// Content streaming
	Content string

	// Tool execution
	ToolCall   *message.ToolCall
	ToolName   string
	ToolResult string
	ToolError  bool

	// Token tracking
	PromptTokens     int64
	CompletionTokens int64

	// Iteration tracking
	Iteration int

	// Error
	Error error

	// Timing
	ResponseTime float64 // seconds
}

// TokenTracker accumulates token usage across iterations
type TokenTracker struct {
	PromptTokens     int64
	CompletionTokens int64
	Iterations       int
}

// Add adds token usage from a provider response
func (t *TokenTracker) Add(usage *provider.TokenUsage) {
	if usage == nil {
		return
	}
	t.PromptTokens += usage.PromptTokens
	t.CompletionTokens += usage.CompletionTokens
}

// Total returns total tokens used
func (t *TokenTracker) Total() int64 {
	return t.PromptTokens + t.CompletionTokens
}
