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
	DefaultMaxTokens     = 65536              // 65K tokens output
	DefaultTimeout       = 10 * time.Minute   // 10 minutes HTTP timeout
	DefaultKeepAlive     = 5 * time.Minute    // 5 minutes keep-alive
	DefaultContextWindow = 131072             // 128K context window
	DefaultMaxRetries    = 3                  // Retry on timeout/connection errors
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
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     DefaultKeepAlive,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 30 * time.Second,
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

func (p *OpenAIProvider) buildMessages(messages []message.Message) []openaiMessage {
	openaiMsgs := make([]openaiMessage, 0, len(messages))
	for _, msg := range messages {
		content := ""
		for _, part := range msg.Parts {
			if tc, ok := part.(message.TextContent); ok {
				content += tc.Text
			}
			if tr, ok := part.(message.ToolResult); ok {
				content += tr.Content
			}
		}
		if content == "" {
			continue
		}
		role := msg.Role
		if role == "tool" {
			role = "user" // Map tool role to user for OpenAI compatibility
		}
		openaiMsgs = append(openaiMsgs, openaiMessage{
			Role:    role,
			Content: content,
		})
	}
	return openaiMsgs
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

func (p *OpenAIProvider) SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error) {
	openaiMsgs := p.buildMessages(messages)

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

		openaiMsgs := p.buildMessages(messages)

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
				eventCh <- Event{Type: EventComplete, TokenUsage: tokenUsage}
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
