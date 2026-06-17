package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fikriaf/ngodeai-cli/internal/message"
)

// Default constants for provider configuration
const (
	DefaultMaxTokens     = 65536            // 65K tokens output
	DefaultTimeout       = 10 * time.Minute // 10 minutes HTTP timeout
	DefaultKeepAlive     = 5 * time.Minute  // 5 minutes keep-alive
	DefaultContextWindow = 131072           // 128K context window
	DefaultMaxRetries    = 3                // Retry on timeout/connection errors
)

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      Model
	httpClient *http.Client
	maxRetries int
}

// newHTTPClient creates a custom HTTP client with proper timeout and keep-alive
func newHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: DefaultKeepAlive,
		}).DialContext,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       DefaultKeepAlive,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
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
			ContextWindow: DefaultContextWindow,
			MaxTokens:     DefaultMaxTokens,
		},
		httpClient: newHTTPClient(DefaultTimeout),
		maxRetries: DefaultMaxRetries,
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
			ContextWindow: DefaultContextWindow,
			MaxTokens:     DefaultMaxTokens, // 65K for custom endpoints
		},
		httpClient: newHTTPClient(DefaultTimeout),
		maxRetries: DefaultMaxRetries,
	}
}

// NewOpenAIWithConfig creates a provider with custom configuration
func NewOpenAIWithConfig(apiKey, modelID, baseURL string, maxTokens, timeoutSec, contextWindow int) *OpenAIProvider {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	if timeoutSec <= 0 {
		timeoutSec = int(DefaultTimeout.Seconds())
	}
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}

	provider := "openai"
	if baseURL != "" && baseURL != "https://api.openai.com/v1" {
		provider = "custom"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model: Model{
			ID:            modelID,
			Name:          modelID,
			Provider:      provider,
			ContextWindow: int64(contextWindow),
			MaxTokens:     int64(maxTokens),
		},
		httpClient: newHTTPClient(time.Duration(timeoutSec) * time.Second),
		maxRetries: DefaultMaxRetries,
	}
}

func (p *OpenAIProvider) Model() Model {
	return p.model
}

// ── Request/Response types ──────────────────────────────────────────────────

type openaiRequest struct {
	Model       string            `json:"model"`
	Messages    []openaiMessage   `json:"messages"`
	MaxTokens   int               `json:"max_tokens"`
	Stream      bool              `json:"stream"`
	Temperature float64           `json:"temperature"`
	Tools       []openaiTool      `json:"tools,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openaiMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []openaiToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int             `json:"index"`
		Message      openaiMessage   `json:"message"`
		FinishReason string          `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Delta        openaiMessage `json:"delta"`
		FinishReason *string       `json:"finish_reason"`
	} `json:"choices"`
}

// ── Helper functions ────────────────────────────────────────────────────────

func (p *OpenAIProvider) buildMessages(messages []message.Message) []openaiMessage {
	openaiMsgs := make([]openaiMessage, 0, len(messages))
	for _, msg := range messages {
		var content strings.Builder
		var toolCalls []openaiToolCall
		var toolCallID string

		for _, part := range msg.Parts {
			switch v := part.(type) {
			case message.TextContent:
				content.WriteString(v.Text)
			case message.ToolCall:
				// This is an assistant message with a tool call
				argsJSON, _ := json.Marshal(v.Args)
				toolCalls = append(toolCalls, openaiToolCall{
					ID:   v.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      v.Name,
						Arguments: string(argsJSON),
					},
				})
			case message.ToolResult:
				// This is a tool result message
				content.WriteString(v.Content)
				toolCallID = v.ToolCallID
			}
		}

		// Skip empty messages (but allow tool results with empty content)
		if content.Len() == 0 && len(toolCalls) == 0 && toolCallID == "" {
			continue
		}

		om := openaiMessage{
			Role:    msg.Role,
			Content: content.String(),
		}

		// Handle assistant messages with tool calls
		if len(toolCalls) > 0 {
			om.ToolCalls = toolCalls
		}

		// Handle tool result messages
		if toolCallID != "" {
			om.Role = "tool"
			om.ToolCallID = toolCallID
		}

		openaiMsgs = append(openaiMsgs, om)
	}
	return openaiMsgs
}

func (p *OpenAIProvider) buildTools(tools []Tool) []openaiTool {
	if len(tools) == 0 {
		return nil
	}
	openaiTools := make([]openaiTool, len(tools))
	for i, t := range tools {
		openaiTools[i] = openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return openaiTools
}

func (p *OpenAIProvider) doRequest(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			time.Sleep(time.Duration(1<<uint(attempt-1)) * 2 * time.Second)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// Retry on timeout/connection errors
			if isRetryableError(err) {
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Retry on 429 (rate limit) and 5xx errors
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("request failed after %d retries: %w", p.maxRetries, lastErr)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "broken pipe")
}

// ── Provider interface implementation ───────────────────────────────────────

func (p *OpenAIProvider) SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error) {
	openaiMsgs := p.buildMessages(messages)
	openaiTools := p.buildTools(tools)

	reqBody := openaiRequest{
		Model:       p.model.ID,
		Messages:    openaiMsgs,
		MaxTokens:   int(p.model.MaxTokens),
		Stream:      false,
		Temperature: 0.7,
		Tools:       openaiTools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
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

	choice := result.Choices[0]
	response := &Response{
		Content: choice.Message.Content,
		TokensUsed: TokenUsage{
			PromptTokens:     int64(result.Usage.PromptTokens),
			CompletionTokens: int64(result.Usage.CompletionTokens),
		},
	}

	// Parse tool calls from response
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"raw": tc.Function.Arguments}
		}
		response.ToolCalls = append(response.ToolCalls, message.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return response, nil
}

func (p *OpenAIProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []Tool) <-chan Event {
	eventCh := make(chan Event, 100)

	go func() {
		defer close(eventCh)

		openaiMsgs := p.buildMessages(messages)
		openaiTools := p.buildTools(tools)

		reqBody := openaiRequest{
			Model:       p.model.ID,
			Messages:    openaiMsgs,
			MaxTokens:   int(p.model.MaxTokens),
			Stream:      true,
			Temperature: 0.7,
			Tools:       openaiTools,
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
		req.Header.Set("Connection", "keep-alive")

		resp, err := p.doRequest(req)
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

		// Use bufio.Scanner for reliable SSE line-by-line reading
		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer size for large chunks
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var tokenUsage *TokenUsage
		// Accumulate tool calls across chunks (they come in pieces)
		var pendingToolCalls []message.ToolCall
		var currentToolCallID string
		var currentToolCallName string
		var currentToolCallArgs strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				// Flush any pending tool call
				if currentToolCallID != "" {
					var args map[string]any
					if err := json.Unmarshal([]byte(currentToolCallArgs.String()), &args); err != nil {
						args = map[string]any{"raw": currentToolCallArgs.String()}
					}
					pendingToolCalls = append(pendingToolCalls, message.ToolCall{
						ID:   currentToolCallID,
						Name: currentToolCallName,
						Args: args,
					})
					eventCh <- Event{
						Type:     EventToolUseStop,
						ToolCall: &pendingToolCalls[len(pendingToolCalls)-1],
					}
				}
				eventCh <- Event{Type: EventComplete, TokenUsage: tokenUsage}
				return
			}

			var chunk openaiStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta

				// Handle content delta
				if delta.Content != "" {
					eventCh <- Event{
						Type:    EventContentDelta,
						Content: delta.Content,
					}
				}

				// Handle tool calls delta
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						// New tool call starting
						if tc.ID != "" {
							// Flush previous tool call if any
							if currentToolCallID != "" {
								var args map[string]any
								if err := json.Unmarshal([]byte(currentToolCallArgs.String()), &args); err != nil {
									args = map[string]any{"raw": currentToolCallArgs.String()}
								}
								pendingToolCalls = append(pendingToolCalls, message.ToolCall{
									ID:   currentToolCallID,
									Name: currentToolCallName,
									Args: args,
								})
								eventCh <- Event{
									Type:     EventToolUseStop,
									ToolCall: &pendingToolCalls[len(pendingToolCalls)-1],
								}
							}
							// Start new tool call
							currentToolCallID = tc.ID
							currentToolCallName = tc.Function.Name
							currentToolCallArgs.Reset()
							eventCh <- Event{
								Type: EventToolUseStart,
								ToolCall: &message.ToolCall{
									ID:   tc.ID,
									Name: tc.Function.Name,
								},
							}
						}
						// Accumulate arguments
						if tc.Function.Arguments != "" {
							currentToolCallArgs.WriteString(tc.Function.Arguments)
						}
					}
				}

				if chunk.Choices[0].FinishReason != nil {
					// Flush any pending tool call
					if currentToolCallID != "" {
						var args map[string]any
						if err := json.Unmarshal([]byte(currentToolCallArgs.String()), &args); err != nil {
							args = map[string]any{"raw": currentToolCallArgs.String()}
						}
						pendingToolCalls = append(pendingToolCalls, message.ToolCall{
							ID:   currentToolCallID,
							Name: currentToolCallName,
							Args: args,
						})
						eventCh <- Event{
							Type:     EventToolUseStop,
							ToolCall: &pendingToolCalls[len(pendingToolCalls)-1],
						}
						currentToolCallID = ""
					}
					eventCh <- Event{Type: EventComplete, TokenUsage: tokenUsage}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("stream read error: %w", err)}
			return
		}

		eventCh <- Event{Type: EventComplete, TokenUsage: tokenUsage}
	}()

	return eventCh
}
