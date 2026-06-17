package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/llm/tools"
	"github.com/fikriaf/ngodeai-cli/internal/message"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// maxAgentIterations prevents infinite tool-use loops
const maxAgentIterations = 10

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

// buildProviderTools converts the agent's tool registry to provider-format tool definitions.
func (a *Agent) buildProviderTools() []provider.Tool {
	providerTools := make([]provider.Tool, 0, len(a.tools))
	for _, t := range a.tools {
		info := t.Info()
		providerTools = append(providerTools, provider.Tool{
			Name:        info.Name,
			Description: info.Description,
			Parameters:  info.Parameters,
		})
	}
	return providerTools
}

// Run executes the agent loop for a given session with tool calling support
func (a *Agent) Run(ctx context.Context, sessionID string, userContent string) (string, error) {
	// Create user message
	_, err := a.messages.Create(sessionID, "user", []message.ContentPart{
		message.TextContent{Text: userContent},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user message: %w", err)
	}

	// Check if we need to compact before LLM call
	if _, err := a.CompactIfNeeded(ctx, sessionID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: compaction check failed: %v\n", err)
	}

	// Build in-memory conversation
	allMessages, err := a.GetCompactedMessages(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to list messages: %w", err)
	}
	conversation := make([]message.Message, len(allMessages))
	copy(conversation, allMessages)

	providerTools := a.buildProviderTools()

	var finalContent string

	// Agent loop with tool calling
	for iteration := 0; iteration < maxAgentIterations; iteration++ {
		resp, err := a.provider.SendMessages(ctx, conversation, providerTools)
		if err != nil {
			return "", fmt.Errorf("provider error: %w", err)
		}

		// Create assistant message with content and tool calls
		parts := make([]message.ContentPart, 0, 1+len(resp.ToolCalls))
		if resp.Content != "" {
			parts = append(parts, message.TextContent{Text: resp.Content})
		}
		for _, tc := range resp.ToolCalls {
			parts = append(parts, tc)
		}
		if len(parts) > 0 {
			if _, err := a.messages.Create(sessionID, "assistant", parts); err != nil {
				return "", fmt.Errorf("failed to save assistant message: %w", err)
			}
		}

		// No tool calls → done
		if len(resp.ToolCalls) == 0 {
			finalContent = resp.Content
			break
		}

		// Build in-memory assistant message
		assistantParts := make([]message.ContentPart, 0, 1+len(resp.ToolCalls))
		if resp.Content != "" {
			assistantParts = append(assistantParts, message.TextContent{Text: resp.Content})
		}
		for _, tc := range resp.ToolCalls {
			assistantParts = append(assistantParts, tc)
		}
		conversation = append(conversation, message.Message{
			Role:  "assistant",
			Parts: assistantParts,
		})

		// Execute each tool
		for _, tc := range resp.ToolCalls {
			var resultContent string
			var isError bool

			tool, ok := a.tools[tc.Name]
			if !ok {
				resultContent = fmt.Sprintf("unknown tool: %s", tc.Name)
				isError = true
			} else {
				result, runErr := tool.Run(ctx, tools.ToolCall{
					CallID: tc.ID,
					Name:   tc.Name,
					Args:   tc.Args,
				})
				if runErr != nil {
					resultContent = fmt.Sprintf("Error: %v", runErr)
					isError = true
				} else {
					resultContent = result.Content
					isError = result.IsError
				}
			}

			// Persist tool result
			a.messages.Create(sessionID, "tool", []message.ContentPart{
				message.ToolResult{
					ToolCallID: tc.ID,
					Content:    resultContent,
					IsError:    isError,
				},
			})

			// Add to in-memory conversation
			conversation = append(conversation, message.Message{
				Role: "tool",
				Parts: []message.ContentPart{
					message.ToolResult{
						ToolCallID: tc.ID,
						Content:    resultContent,
						IsError:    isError,
					},
				},
			})
		}
	}

	return finalContent, nil
}

// StreamRun executes the full agent loop with streaming: content deltas are
// pushed to the returned content channel as they arrive, tool calls are
// executed automatically, and the loop continues until the model produces a
// final text-only response (or the iteration cap is hit).
//
// The caller reads from the content channel for real-time text, and from the
// error channel for a terminal error (nil on success).  Both channels are
// closed when the run finishes.
func (a *Agent) StreamRun(ctx context.Context, sessionID string, userContent string) (<-chan string, <-chan error) {
	contentCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		// Close contentCh first, errCh second (LIFO) so the TUI reader
		// always sees contentCh close before errCh, avoiding a race
		// where a pending error could be silently dropped.
		defer close(errCh)
		defer close(contentCh)

		// ── Persist user message ──────────────────────────────────
		_, err := a.messages.Create(sessionID, "user", []message.ContentPart{
			message.TextContent{Text: userContent},
		})
		if err != nil {
			errCh <- fmt.Errorf("failed to create user message: %w", err)
			return
		}

		// ── Build in-memory conversation ──────────────────────────
		// We keep our own message slice so that tool-call and
		// tool-result parts are available to the provider without
		// round-tripping through the DB (whose ContentPart
		// deserialiser may not handle all part types yet).

		// Check if we need to compact before LLM call
		if _, err := a.CompactIfNeeded(ctx, sessionID); err != nil {
			// Log but continue - we don't want to fail the request
			fmt.Fprintf(os.Stderr, "warning: compaction check failed: %v\n", err)
		}

		// Get messages (using compacted version if available)
		dbMessages, err := a.GetCompactedMessages(sessionID)
		if err != nil {
			errCh <- fmt.Errorf("failed to list messages: %w", err)
			return
		}
		conversation := make([]message.Message, len(dbMessages))
		copy(conversation, dbMessages)

		providerTools := a.buildProviderTools()

		// ── Agent loop ────────────────────────────────────────────
		for iteration := 0; iteration < maxAgentIterations; iteration++ {

			// Stream one turn from the provider
			eventCh := a.provider.StreamResponse(ctx, conversation, providerTools)

			var fullContent strings.Builder
			var toolCalls []message.ToolCall

			for event := range eventCh {
				switch event.Type {
				case provider.EventContentDelta:
					fullContent.WriteString(event.Content)
					contentCh <- event.Content

				case provider.EventToolUseStart:
					// Let the TUI know a tool call is incoming
					if event.ToolCall != nil {
						contentCh <- fmt.Sprintf("\n⚡ Running: %s\n\n", event.ToolCall.Name)
					}

				case provider.EventToolUseStop:
					if event.ToolCall != nil {
						toolCalls = append(toolCalls, *event.ToolCall)
					}

				case provider.EventError:
					errCh <- event.Error
					return
				}
			}

			// ── Persist assistant message ─────────────────────────
			if fullContent.Len() > 0 || len(toolCalls) > 0 {
				parts := make([]message.ContentPart, 0, 1+len(toolCalls))
				if fullContent.Len() > 0 {
					parts = append(parts, message.TextContent{Text: fullContent.String()})
				}
				// Save tool-call parts for future message-history
				// replay (ignored by providers that don't support
				// them yet, but structurally correct).
				for _, tc := range toolCalls {
					parts = append(parts, tc)
				}
				if _, err := a.messages.Create(sessionID, "assistant", parts); err != nil {
					errCh <- fmt.Errorf("failed to save assistant message: %w", err)
					return
				}
			}

			// ── No tool calls → final response, we're done ───────
			if len(toolCalls) == 0 {
				return
			}

			// ── Build in-memory assistant message with tool calls ─
			assistantParts := make([]message.ContentPart, 0, 1+len(toolCalls))
			if fullContent.Len() > 0 {
				assistantParts = append(assistantParts, message.TextContent{Text: fullContent.String()})
			}
			for _, tc := range toolCalls {
				assistantParts = append(assistantParts, tc)
			}
			conversation = append(conversation, message.Message{
				Role:  "assistant",
				Parts: assistantParts,
			})

			// ── Execute each tool and collect results ─────────────
			for _, tc := range toolCalls {
				// Notify TUI (if not already notified via EventToolUseStart)
				contentCh <- fmt.Sprintf("\n⚡ Running: %s\n\n", tc.Name)

				var resultContent string
				var isError bool

				tool, ok := a.tools[tc.Name]
				if !ok {
					resultContent = fmt.Sprintf("unknown tool: %s", tc.Name)
					isError = true
				} else {
					result, runErr := tool.Run(ctx, tools.ToolCall{
						CallID: tc.ID,
						Name:   tc.Name,
						Args:   tc.Args,
					})
					if runErr != nil {
						resultContent = fmt.Sprintf("Error: %v", runErr)
						isError = true
					} else {
						resultContent = result.Content
						isError = result.IsError
					}
				}

				// Persist tool result
				a.messages.Create(sessionID, "tool", []message.ContentPart{
					message.ToolResult{
						ToolCallID: tc.ID,
						Content:    resultContent,
						IsError:    isError,
					},
				})

				// Add tool result to in-memory conversation
				conversation = append(conversation, message.Message{
					Role: "tool",
					Parts: []message.ContentPart{
						message.ToolResult{
							ToolCallID: tc.ID,
							Content:    resultContent,
							IsError:    isError,
						},
					},
				})
			}

			// Loop back: the provider will see the tool results and
			// produce a follow-up response.
		}

		// Hit the iteration cap
		errCh <- fmt.Errorf("agent loop exceeded maximum iterations (%d)", maxAgentIterations)
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
