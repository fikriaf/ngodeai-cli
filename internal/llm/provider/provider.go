package provider

import (
	"context"

	"github.com/fikriaf/ngodeai-cli/internal/message"
)

// Provider is the interface for LLM providers
type Provider interface {
	// SendMessages sends messages and returns a response
	SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error)

	// StreamResponse streams response events
	StreamResponse(ctx context.Context, messages []message.Message, tools []Tool) <-chan Event

	// Model returns the current model info
	Model() Model
}

// Tool is a simplified tool interface for providers
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Response is a complete LLM response
type Response struct {
	Content    string
	ToolCalls  []message.ToolCall
	TokensUsed TokenUsage
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int64
	CompletionTokens int64
}

// Event is a streaming event from the provider
type Event struct {
	Type       string
	Content    string
	ToolCall   *message.ToolCall
	Error      error
	TokenUsage *TokenUsage
}

// Event types
const (
	EventContentStart  = "content_start"
	EventContentDelta  = "content_delta"
	EventToolUseStart  = "tool_use_start"
	EventToolUseStop   = "tool_use_stop"
	EventComplete      = "complete"
	EventError         = "error"
)

// Model holds model metadata
type Model struct {
	ID           string
	Name         string
	Provider     string
	ContextWindow int64
	MaxTokens    int64
}
