package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/message"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   Model
}

// NewOpenAI creates a new OpenAI provider
func NewOpenAI(apiKey string, modelID string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		model: Model{
			ID:            modelID,
			Name:          modelID,
			Provider:      "openai",
			ContextWindow: 128000,
			MaxTokens:     4096,
		},
	}
}

// NewOpenAIWithBaseURL creates an OpenAI-compatible provider with custom base URL
func NewOpenAIWithBaseURL(apiKey string, modelID string, baseURL string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model: Model{
			ID:            modelID,
			Name:          modelID,
			Provider:      "custom",
			ContextWindow: 128000,
			MaxTokens:     4096,
		},
	}
}

func (p *OpenAIProvider) Model() Model {
	return p.model
}

// openaiRequest is the request payload
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the response payload
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiStreamChunk is a streaming chunk
type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error) {
	// Convert messages to OpenAI format
	openaiMsgs := make([]openaiMessage, 0, len(messages))
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
		openaiMsgs = append(openaiMsgs, openaiMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	// Build request
	reqBody := openaiRequest{
		Model:       p.model.ID,
		Messages:    openaiMsgs,
		MaxTokens:   int(p.model.MaxTokens),
		Stream:      false,
		Temperature: 0.7,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &Response{
		Content: result.Choices[0].Message.Content,
		TokensUsed: TokenUsage{
			PromptTokens:     int64(result.Usage.PromptTokens),
			CompletionTokens: int64(result.Usage.CompletionTokens),
		},
	}, nil
}

func (p *OpenAIProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []Tool) <-chan Event {
	eventCh := make(chan Event, 100)

	go func() {
		defer close(eventCh)

		// Convert messages
		openaiMsgs := make([]openaiMessage, 0, len(messages))
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
			openaiMsgs = append(openaiMsgs, openaiMessage{
				Role:    msg.Role,
				Content: content,
			})
		}

		// Build streaming request
		reqBody := openaiRequest{
			Model:       p.model.ID,
			Messages:    openaiMsgs,
			MaxTokens:   int(p.model.MaxTokens),
			Stream:      true,
			Temperature: 0.7,
		}

		bodyBytes, _ := json.Marshal(reqBody)

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(bodyBytes)))
		if err != nil {
			eventCh <- Event{Type: EventError, Error: err}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
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

		// Read SSE stream
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

			// Parse SSE data lines
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					eventCh <- Event{Type: EventComplete}
					return
				}

				var chunk openaiStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}

				if len(chunk.Choices) > 0 {
					if chunk.Choices[0].Delta.Content != "" {
						eventCh <- Event{
							Type:    EventContentDelta,
							Content: chunk.Choices[0].Delta.Content,
						}
					}
					if chunk.Choices[0].FinishReason != nil {
						eventCh <- Event{Type: EventComplete}
						return
					}
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
