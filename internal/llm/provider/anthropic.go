package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/message"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude
type AnthropicProvider struct {
	apiKey string
	model  Model
}

// NewAnthropic creates a new Anthropic provider
func NewAnthropic(apiKey string, modelID string) *AnthropicProvider {
	name := modelID
	if name == "" {
		name = "claude-3-5-sonnet-20241022"
	}

	return &AnthropicProvider{
		apiKey: apiKey,
		model: Model{
			ID:            name,
			Name:          name,
			Provider:      "anthropic",
			ContextWindow: 200000,
			MaxTokens:     8192,
		},
	}
}

func (p *AnthropicProvider) Model() Model {
	return p.model
}

// anthropicRequest is the request payload
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the response payload
type anthropicResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (p *AnthropicProvider) SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error) {
	// Convert messages
	anthropicMsgs := make([]anthropicMessage, 0, len(messages))
	for _, msg := range messages {
		content := ""
		for _, part := range msg.Parts {
			if tc, ok := part.(message.TextContent); ok {
				content += tc.Text
			}
		}
		if content == "" {
			continue
		}
		// Anthropic uses 'user' and 'assistant', not 'system' in messages array
		role := msg.Role
		if role == "system" {
			role = "user"
		}
		anthropicMsgs = append(anthropicMsgs, anthropicMessage{
			Role:    role,
			Content: content,
		})
	}

	reqBody := anthropicRequest{
		Model:     p.model.ID,
		MaxTokens: int(p.model.MaxTokens),
		Messages:  anthropicMsgs,
		Stream:    false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	content := ""
	for _, c := range result.Content {
		content += c.Text
	}

	return &Response{
		Content: content,
		TokensUsed: TokenUsage{
			PromptTokens:     int64(result.Usage.InputTokens),
			CompletionTokens: int64(result.Usage.OutputTokens),
		},
	}, nil
}

func (p *AnthropicProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []Tool) <-chan Event {
	eventCh := make(chan Event, 100)

	go func() {
		defer close(eventCh)

		// Convert messages
		anthropicMsgs := make([]anthropicMessage, 0, len(messages))
		for _, msg := range messages {
			content := ""
			for _, part := range msg.Parts {
				if tc, ok := part.(message.TextContent); ok {
					content += tc.Text
				}
			}
			if content == "" {
				continue
			}
			role := msg.Role
			if role == "system" {
				role = "user"
			}
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    role,
				Content: content,
			})
		}

		reqBody := anthropicRequest{
			Model:     p.model.ID,
			MaxTokens: int(p.model.MaxTokens),
			Messages:  anthropicMsgs,
			Stream:    true,
		}

		bodyBytes, _ := json.Marshal(reqBody)

		req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			eventCh <- Event{Type: EventError, Error: err}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))}
			return
		}

		eventCh <- Event{Type: EventContentStart}

		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if err != nil && err != io.EOF {
				eventCh <- Event{Type: EventError, Error: err}
				return
			}
			if n == 0 {
				break
			}

			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")

				var event map[string]interface{}
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					continue
				}

				eventType, _ := event["type"].(string)
				switch eventType {
				case "content_block_delta":
					if delta, ok := event["delta"].(map[string]interface{}); ok {
						if text, ok := delta["text"].(string); ok {
							eventCh <- Event{Type: EventContentDelta, Content: text}
						}
					}
				case "message_stop":
					eventCh <- Event{Type: EventComplete}
					return
				}
			}

			if err == io.EOF {
				break
			}
		}

		eventCh <- Event{Type: EventComplete}
	}()

	return eventCh
}
