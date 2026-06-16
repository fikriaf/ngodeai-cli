package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fikriaf/ngodeai-cli/internal/message"
)

const (
	defaultContextWindow    = 200000 // Default context window size
	maxRecentMessages       = 10     // Keep last N messages intact
	tokenSafetyMargin       = 0.90   // Trigger compacting at 90% of context window
	minHistoryForCompaction = 5      // Minimum number of messages before allowing compaction
)

// tokenEstimator provides simple token counting for estimation
type tokenEstimator struct{}

// estimateTokensApproximate returns an approximate token count
// This is a rough heuristic: ~4 characters ~= 1 token
func (e *tokenEstimator) estimate(text string) int64 {
	return int64(len(text)) / 4
}

// summarizeConversation creates a summary of conversation history
// preserving only the most recent messages and summarizing older ones
func (a *Agent) summarizeConversation(ctx context.Context, messages []message.Message) ([]byte, error) {
	if len(messages) <= maxRecentMessages {
		return nil, fmt.Errorf("no need to summarize - message count within threshold")
	}

	recentStart := len(messages) - maxRecentMessages
	oldMsgs := messages[:recentStart]

	var sb strings.Builder

	sb.WriteString("=== CONVERSATION SUMMARY ===\n")
	sb.WriteString(fmt.Sprintf("Summary created at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Original message count: %d\n", len(messages)))
	sb.WriteString(fmt.Sprintf("Recent messages preserved: %d\n\n", len(messages)-recentStart))

	// Generate chronological summary
	for i, msg := range oldMsgs {
		prefix := "System"
		switch msg.Role {
		case "user":
			prefix = "User"
		case "assistant":
			prefix = "Assistant"
		case "tool":
			prefix = "Tool Result"
		}

		content := ""
		for _, part := range msg.Parts {
			if textContent, ok := part.(message.TextContent); ok {
				content += textContent.Text
			}
		}

		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", i+1, prefix, truncateMessage(content, 200)))
	}

	sb.WriteString("\n=== END SUMMARY ===\n\n")

	return []byte(sb.String()), nil
}

// truncateMessage truncates content if too long while preserving readability
func truncateMessage(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "... (truncated)"
}

// buildReducedConversation creates a conversation list that combines:
// 1. A summary message representing older history
// 2. The most recent messages
func (a *Agent) buildReducedConversation(summaryText string, recentMessages []message.Message, sessionID string) []message.Message {
	result := make([]message.Message, 0, 1+len(recentMessages))

	// Add summary as first message
	summaryPart := message.TextContent{Text: summaryText}
	summaryMsg := message.Message{
		SessionID: sessionID,
		Role:      "system",
		Parts:     []message.ContentPart{summaryPart},
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	result = append(result, summaryMsg)

	// Append recent messages
	result = append(result, recentMessages...)

	return result
}

// CompactIfNeeded checks if token usage is approaching the context limit
// and triggers summarization if necessary.
// Returns true if compaction was performed, false otherwise.
func (a *Agent) CompactIfNeeded(ctx context.Context, sessionID string) (bool, error) {
	// Get current model context window
	model := a.provider.Model()
	contextWindow := model.ContextWindow
	if contextWindow == 0 {
		contextWindow = defaultContextWindow
	}

	threshold := int64(float64(contextWindow) * tokenSafetyMargin)

	messages, err := a.messages.List(sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to list messages: %w", err)
	}

	// Need enough messages to be worth summarizing
	if len(messages) < minHistoryForCompaction {
		return false, nil
	}

	totalTokens := int64(0)
	tokenEst := &tokenEstimator{}

	// Estimate total tokens
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if textContent, ok := part.(message.TextContent); ok {
				totalTokens += tokenEst.estimate(textContent.Text)
			}
		}
	}

	// Check if we need to compact
	if totalTokens < threshold {
		return false, nil
	}

	// Perform compaction
	summaryBytes, err := a.summarizeConversation(ctx, messages)
	if err != nil {
		// Don't fail the request if summarization fails
		return false, nil
	}

	// Create summary message in database
	summaryMsg, err := a.messages.Create(sessionID, "system", []message.ContentPart{
		message.TextContent{Text: string(summaryBytes)},
	})
	if err != nil {
		return false, fmt.Errorf("failed to create summary message: %w", err)
	}

	// Update session with summary reference
	if err := a.sessions.UpdateSummaryMessageID(sessionID, summaryMsg.ID); err != nil {
		// Log but don't fail - the summary is still saved
		fmt.Fprintf(os.Stderr, "warning: failed to update session summary reference: %v\n", err)
	}

	return true, nil
}

// GetCompactedMessages returns messages suitable for sending to the LLM,
// using summary + recent messages if compaction has occurred.
func (a *Agent) GetCompactedMessages(sessionID string) ([]message.Message, error) {
	sess, err := a.sessions.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	messages, err := a.messages.List(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	// If no summary exists or no summary message ID, return all messages
	if !sess.SummaryMessageID.Valid || sess.SummaryMessageID.String == "" {
		return messages, nil
	}

	// Find the summary message and messages after it
	summaryID := sess.SummaryMessageID.String
	var summaryMsg *message.Message
	var recentMsgs []message.Message
	foundSummary := false

	for i := range messages {
		if messages[i].ID == summaryID {
			summaryMsg = &messages[i]
			foundSummary = true
			continue
		}
		if foundSummary {
			recentMsgs = append(recentMsgs, messages[i])
		}
	}

	// If summary not found, return all messages
	if summaryMsg == nil {
		return messages, nil
	}

	// Build reduced conversation: summary + recent messages
	result := make([]message.Message, 0, 1+len(recentMsgs))
	result = append(result, *summaryMsg)
	result = append(result, recentMsgs...)

	return result, nil
}
