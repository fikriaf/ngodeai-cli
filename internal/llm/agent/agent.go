package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/llm/tools"
	"github.com/fikriaf/ngodeai-cli/internal/message"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// Agent orchestrates the LLM + tool-use loop
type Agent struct {
	provider provider.Provider
	tools    map[string]tools.BaseTool
	sessions *session.Service
	messages *message.Service
}

// New creates a new agent
func New(p provider.Provider, toolList []tools.BaseTool, sessions *session.Service, messages *message.Service) *Agent {
	toolMap := make(map[string]tools.BaseTool)
	for _, t := range toolList {
		toolMap[t.Info().Name] = t
	}

	return &Agent{
		provider: p,
		tools:    toolMap,
		sessions: sessions,
		messages: messages,
	}
}

// Run executes the agent loop for a given session
func (a *Agent) Run(ctx context.Context, sessionID string, userContent string) (string, error) {
	// Create user message
	userMsg, err := a.messages.Create(sessionID, "user", []message.ContentPart{
		message.TextContent{Text: userContent},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user message: %w", err)
	}

	// Get all messages for this session
	allMessages, err := a.messages.List(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to list messages: %w", err)
	}

	// Build tool list for provider
	providerTools := make([]provider.Tool, 0, len(a.tools))
	for _, t := range a.tools {
		info := t.Info()
		providerTools = append(providerTools, provider.Tool{
			Name:        info.Name,
			Description: info.Description,
			Parameters:  info.Parameters,
		})
	}

	// Simple single-turn for now (no tool loop yet)
	resp, err := a.provider.SendMessages(ctx, allMessages, providerTools)
	if err != nil {
		return "", fmt.Errorf("provider error: %w", err)
	}

	// Create assistant message
	_, err = a.messages.Create(sessionID, "assistant", []message.ContentPart{
		message.TextContent{Text: resp.Content},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create assistant message: %w", err)
	}

	_ = userMsg // used for tracking

	return resp.Content, nil
}

// StreamRun executes the agent with streaming
func (a *Agent) StreamRun(ctx context.Context, sessionID string, userContent string) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(contentCh)
		defer close(errCh)

		// Create user message
		_, err := a.messages.Create(sessionID, "user", []message.ContentPart{
			message.TextContent{Text: userContent},
		})
		if err != nil {
			errCh <- fmt.Errorf("failed to create user message: %w", err)
			return
		}

		// Get all messages
		allMessages, err := a.messages.List(sessionID)
		if err != nil {
			errCh <- fmt.Errorf("failed to list messages: %w", err)
			return
		}

		// Build tool list
		providerTools := make([]provider.Tool, 0, len(a.tools))
		for _, t := range a.tools {
			info := t.Info()
			providerTools = append(providerTools, provider.Tool{
				Name:        info.Name,
				Description: info.Description,
				Parameters:  info.Parameters,
			})
		}

		// Stream response
		eventCh := a.provider.StreamResponse(ctx, allMessages, providerTools)
		var fullContent strings.Builder

		for event := range eventCh {
			switch event.Type {
			case provider.EventContentDelta:
				fullContent.WriteString(event.Content)
				contentCh <- event.Content
			case provider.EventError:
				errCh <- event.Error
				return
			case provider.EventComplete:
				// Save the complete message
				_, err := a.messages.Create(sessionID, "assistant", []message.ContentPart{
					message.TextContent{Text: fullContent.String()},
				})
				if err != nil {
					errCh <- fmt.Errorf("failed to save message: %w", err)
					return
				}
				return
			}
		}
	}()

	return contentCh, errCh
}

// Provider returns the current provider
func (a *Agent) Provider() provider.Provider {
	return a.provider
}

// SetProvider changes the provider
func (a *Agent) SetProvider(p provider.Provider) {
	a.provider = p
}
