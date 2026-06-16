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

// GeminiProvider implements the Provider interface for Google Gemini
type GeminiProvider struct {
	apiKey  string
	baseURL string
	model   Model
}

// NewGemini creates a new Gemini provider
func NewGemini(apiKey string, modelID string) *GeminiProvider {
	name := modelID
	if name == "" {
		name = "gemini-2.0-flash"
	}

	return &GeminiProvider{
		apiKey:  apiKey,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		model: Model{
			ID:            name,
			Name:          name,
			Provider:      "gemini",
			ContextWindow: 1048576,
			MaxTokens:     8192,
		},
	}
}

func (p *GeminiProvider) Model() Model {
	return p.model
}

// --- Request types ---

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// --- Response types ---

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// convertMessages converts internal messages to Gemini format.
// Gemini uses "user" and "model" roles. System messages are mapped to "user".
func (p *GeminiProvider) convertMessages(messages []message.Message) []geminiContent {
	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		text := ""
		for _, part := range msg.Parts {
			if tc, ok := part.(message.TextContent); ok {
				text += tc.Text
			}
		}
		if text == "" {
			continue
		}

		// Gemini uses "user" and "model"; map system/tool to "user"
		role := msg.Role
		switch role {
		case "assistant":
			role = "model"
		case "system", "tool":
			role = "user"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: text}},
		})
	}
	return contents
}

// buildURL constructs the API endpoint URL
func (p *GeminiProvider) buildURL(action string, sse bool) string {
	url := fmt.Sprintf("%s/models/%s:%s", p.baseURL, p.model.ID, action)
	sep := "?"
	if sse {
		url += sep + "alt=sse"
		sep = "&"
	}
	url += sep + "key=" + p.apiKey
	return url
}

func (p *GeminiProvider) SendMessages(ctx context.Context, messages []message.Message, tools []Tool) (*Response, error) {
	contents := p.convertMessages(messages)

	reqBody := geminiRequest{
		Contents: contents,
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: int(p.model.MaxTokens),
			Temperature:     0.7,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.buildURL("generateContent", false)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// Extract text from parts
	var content strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		content.WriteString(part.Text)
	}

	return &Response{
		Content: content.String(),
		TokensUsed: TokenUsage{
			PromptTokens:     int64(result.UsageMetadata.PromptTokenCount),
			CompletionTokens: int64(result.UsageMetadata.CandidatesTokenCount),
		},
	}, nil
}

func (p *GeminiProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []Tool) <-chan Event {
	eventCh := make(chan Event, 100)

	go func() {
		defer close(eventCh)

		contents := p.convertMessages(messages)

		reqBody := geminiRequest{
			Contents: contents,
			GenerationConfig: geminiGenerationConfig{
				MaxOutputTokens: int(p.model.MaxTokens),
				Temperature:     0.7,
			},
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		url := p.buildURL("streamGenerateContent", true)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			eventCh <- Event{Type: EventError, Error: fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(body))}
			return
		}

		eventCh <- Event{Type: EventContentStart}

		// Read SSE stream — Gemini with ?alt=sse emits "data: {...}\n\n" events
		buf := make([]byte, 4096)
		var pending strings.Builder
		for {
			n, readErr := resp.Body.Read(buf)
			if n == 0 && readErr != nil {
				if readErr != io.EOF {
					eventCh <- Event{Type: EventError, Error: readErr}
				}
				break
			}

			if n > 0 {
				pending.Write(buf[:n])

				// Process complete SSE lines
				for {
					line, rest, found := strings.Cut(pending.String(), "\n")
					if !found {
						break
					}
					pending.Reset()
					pending.WriteString(rest)

					line = strings.TrimSpace(line)
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					data := strings.TrimPrefix(line, "data: ")

					var chunk geminiResponse
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						continue
					}

					// Emit text deltas
					for _, candidate := range chunk.Candidates {
						for _, part := range candidate.Content.Parts {
							if part.Text != "" {
								eventCh <- Event{
									Type:    EventContentDelta,
									Content: part.Text,
								}
							}
						}

						if candidate.FinishReason == "STOP" || candidate.FinishReason == "MAX_TOKENS" {
							// Emit final token usage if available
							if chunk.UsageMetadata.TotalTokenCount > 0 {
								usage := TokenUsage{
									PromptTokens:     int64(chunk.UsageMetadata.PromptTokenCount),
									CompletionTokens: int64(chunk.UsageMetadata.CandidatesTokenCount),
								}
								eventCh <- Event{Type: EventComplete, TokenUsage: &usage}
								return
							}
						}
					}
				}
			}

			if readErr == io.EOF {
				break
			}
		}

		eventCh <- Event{Type: EventComplete}
	}()

	return eventCh
}
