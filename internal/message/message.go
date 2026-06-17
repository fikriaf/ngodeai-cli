package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fikriaf/ngodeai-cli/internal/pubsub"
	"github.com/google/uuid"
)

// Message represents a conversation message
type Message struct {
	ID        string
	SessionID string
	Role      string // user, assistant, tool, system
	Parts     []ContentPart
	Model     sql.NullString
	CreatedAt int64
	UpdatedAt int64
}

// ContentPart is a polymorphic message content piece
type ContentPart interface {
	isPart()
	Type() string
}

// TextContent is plain text
type TextContent struct {
	Kind string `json:"type"`
	Text string `json:"text"`
}

func (t TextContent) isPart() {}
func (t TextContent) Type() string { return "text" }

// ToolCall represents an LLM-initiated tool invocation
type ToolCall struct {
	Kind string         `json:"type"`
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func (t ToolCall) isPart() {}
func (t ToolCall) Type() string { return "tool_call" }

// ToolResult is the result of a tool execution
type ToolResult struct {
	Kind       string `json:"type"`
	ToolCallID string `json:"toolCallId"`
	Content    string `json:"content"`
	IsError    bool   `json:"isError"`
}

func (t ToolResult) isPart() {}
func (t ToolResult) Type() string { return "tool_result" }

// Service provides message CRUD operations
type Service struct {
	db     *sql.DB
	broker *pubsub.Broker[Event]
}

// Event is a message event
type Event struct {
	Type    string
	Message Message
}

// NewService creates a new message service
func NewService(db *sql.DB) *Service {
	return &Service{
		db:     db,
		broker: pubsub.NewBroker[Event](),
	}
}

// Create creates a new message
func (s *Service) Create(sessionID string, role string, parts []ContentPart) (Message, error) {
	id := uuid.New().String()
	now := time.Now().Unix()

	// Ensure each part has a type field, then serialize
	type typedPart struct {
		Type       string         `json:"type"`
		Text       string         `json:"text,omitempty"`
		ID         string         `json:"id,omitempty"`
		Name       string         `json:"name,omitempty"`
		Args       map[string]any `json:"args,omitempty"`
		ToolCallID string         `json:"toolCallId,omitempty"`
		Content    string         `json:"content,omitempty"`
		IsError    bool           `json:"isError,omitempty"`
	}

	var serializable []typedPart
	for _, p := range parts {
		switch v := p.(type) {
		case TextContent:
			serializable = append(serializable, typedPart{Type: "text", Text: v.Text})
		case ToolCall:
			serializable = append(serializable, typedPart{Type: "tool_call", ID: v.ID, Name: v.Name, Args: v.Args})
		case ToolResult:
			serializable = append(serializable, typedPart{Type: "tool_result", ToolCallID: v.ToolCallID, Content: v.Content, IsError: v.IsError})
		}
	}

	partsJSON, err := json.Marshal(serializable)
	if err != nil {
		return Message{}, fmt.Errorf("failed to serialize parts: %w", err)
	}

	msg := Message{
		ID:        id,
		SessionID: sessionID,
		Role:      role,
		Parts:     parts,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = s.db.Exec(`
		INSERT INTO messages (id, session_id, role, parts, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, sessionID, role, string(partsJSON), now, now)
	if err != nil {
		return Message{}, fmt.Errorf("failed to create message: %w", err)
	}

	s.broker.Publish(Event{Type: pubsub.EventCreated, Message: msg})
	return msg, nil
}

// List retrieves all messages for a session
func (s *Service) List(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, parts, model, created_at, updated_at
		FROM messages WHERE session_id = ?
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var partsJSON string
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &partsJSON, &msg.Model, &msg.CreatedAt, &msg.UpdatedAt); err != nil {
			return nil, err
		}

		// Deserialize parts with custom handling for polymorphic types
		var rawParts []json.RawMessage
		if err := json.Unmarshal([]byte(partsJSON), &rawParts); err != nil {
			return nil, fmt.Errorf("failed to deserialize raw parts: %w", err)
		}

		for _, raw := range rawParts {
			var partMap map[string]json.RawMessage
			if err := json.Unmarshal(raw, &partMap); err != nil {
				// Fallback: try as TextContent without type field
				var tc TextContent
				if err2 := json.Unmarshal(raw, &tc); err2 == nil {
					tc.Kind = "text"
					msg.Parts = append(msg.Parts, tc)
				}
				continue
			}

			var partType string
			if t, ok := partMap["type"]; ok {
				json.Unmarshal(t, &partType)
			}

			switch partType {
			case "text":
				var tc TextContent
				json.Unmarshal(raw, &tc)
				msg.Parts = append(msg.Parts, tc)
			case "tool_call":
				var tc ToolCall
				json.Unmarshal(raw, &tc)
				msg.Parts = append(msg.Parts, tc)
			case "tool_result":
				var tr ToolResult
				json.Unmarshal(raw, &tr)
				msg.Parts = append(msg.Parts, tr)
			default:
				// Try as TextContent (legacy format)
				var tc TextContent
				if err := json.Unmarshal(raw, &tc); err == nil {
					tc.Kind = "text"
					msg.Parts = append(msg.Parts, tc)
				}
			}
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

// Subscribe returns a channel for message events
func (s *Service) Subscribe(ctx context.Context) <-chan Event {
	return s.broker.Subscribe(ctx)
}
