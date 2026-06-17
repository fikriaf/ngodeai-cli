package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fikriaf/ngodeai-cli/internal/pubsub"
	"github.com/google/uuid"
)

// Session represents a conversation session
type Session struct {
	ID                string
	ParentSessionID   sql.NullString
	Title             string
	SummaryMessageID  sql.NullString
	MessageCount      int64
	PromptTokens      int64
	CompletionTokens  int64
	Cost              float64
	CreatedAt         int64
	UpdatedAt         int64
}

// Service provides session CRUD operations
type Service struct {
	db     *sql.DB
	broker *pubsub.Broker[Event]
}

// Event is a session event
type Event struct {
	Type    string
	Session Session
}

// NewService creates a new session service
func NewService(db *sql.DB) *Service {
	return &Service{
		db:     db,
		broker: pubsub.NewBroker[Event](),
	}
}

// Create creates a new session
func (s *Service) Create(title string) (Session, error) {
	id := uuid.New().String()
	now := time.Now().Unix()

	sess := Session{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(`
		INSERT INTO sessions (id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, id, title, now, now)
	if err != nil {
		return Session{}, fmt.Errorf("failed to create session: %w", err)
	}

	s.broker.Publish(Event{Type: pubsub.EventCreated, Session: sess})
	return sess, nil
}

// Get retrieves a session by ID
func (s *Service) Get(id string) (Session, error) {
	var sess Session
	err := s.db.QueryRow(`
		SELECT id, parent_session_id, title, summary_message_id,
			   message_count, prompt_tokens, completion_tokens, cost,
			   created_at, updated_at
		FROM sessions WHERE id = ?
	`, id).Scan(
		&sess.ID, &sess.ParentSessionID, &sess.Title, &sess.SummaryMessageID,
		&sess.MessageCount, &sess.PromptTokens, &sess.CompletionTokens, &sess.Cost,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return Session{}, fmt.Errorf("failed to get session: %w", err)
	}
	return sess, nil
}

// List returns all top-level sessions
func (s *Service) List() ([]Session, error) {
	rows, err := s.db.Query(`
		SELECT id, parent_session_id, title, summary_message_id,
			   message_count, prompt_tokens, completion_tokens, cost,
			   created_at, updated_at
		FROM sessions
		WHERE parent_session_id IS NULL
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(
			&sess.ID, &sess.ParentSessionID, &sess.Title, &sess.SummaryMessageID,
			&sess.MessageCount, &sess.PromptTokens, &sess.CompletionTokens, &sess.Cost,
			&sess.CreatedAt, &sess.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// Delete removes a session
func (s *Service) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	s.broker.Publish(Event{Type: pubsub.EventDeleted, Session: Session{ID: id}})
	return nil
}

// UpdateSummaryMessageID updates the summary_message_id for a session
func (s *Service) UpdateSummaryMessageID(sessionID string, summaryMessageID string) error {
	_, err := s.db.Exec(`
		UPDATE sessions 
		SET summary_message_id = ?, updated_at = ?
		WHERE id = ?
	`, summaryMessageID, time.Now().Unix(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update summary message ID: %w", err)
	}
	return nil
}

// UpdateTitle updates the title for a session
func (s *Service) UpdateTitle(sessionID string, title string) error {
	_, err := s.db.Exec(`
		UPDATE sessions 
		SET title = ?, updated_at = ?
		WHERE id = ?
	`, title, time.Now().Unix(), sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session title: %w", err)
	}
	return nil
}

// Subscribe returns a channel for session events
func (s *Service) Subscribe(ctx context.Context) <-chan Event {
	return s.broker.Subscribe(ctx)
}
