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
}

// TextContent is plain text
type TextContent struct {
	Text string `json:"text"`
}

func (t TextContent) isPart() {}

// ToolCall represents an LLM-initiated tool invocation
type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func (t ToolCall) isPart() {}

// ToolResult is the result of a tool execution
type ToolResult struct {
	ToolCallID string `json:"toolCallId"`
	Content    string `json:"content"`
	IsError    bool   `json:"isError"`
}

func (t ToolResult) isPart() {}

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

	// Serialize parts to JSON
	partsJSON, err := json.Marshal(parts)
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

		// Deserialize parts
		if err := json.Unmarshal([]byte(partsJSON), &msg.Parts); err != nil {
			return nil, fmt.Errorf("failed to deserialize parts: %w", err)
		}

		messages = append(messages, msg)
	}
	return messages, nil
}

// Subscribe returns a channel for message events
func (s *Service) Subscribe(ctx context.Context) <-chan Event {
	return s.broker.Subscribe(ctx)
}
