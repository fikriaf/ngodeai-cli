package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3"
	"github.com/pressly/goose/v3"
)

// Connection wraps the SQL database connection
type Connection struct {
	db   *sql.DB
	path string
}

// Connect opens the SQLite database and runs migrations
func Connect(dataDir string) (*Connection, error) {
	dbPath := filepath.Join(dataDir, "ngodeai.db")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA cache_size=-8000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA page_size=4096",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &Connection{db: db, path: dbPath}, nil
}

func runMigrations(db *sql.DB) error {
	// For now, create tables directly
	// In production, use goose with embedded SQL files
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			parent_session_id TEXT REFERENCES sessions(id),
			title TEXT NOT NULL DEFAULT 'New Session',
			summary_message_id TEXT,
			message_count INTEGER DEFAULT 0,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			cost REAL DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role TEXT NOT NULL CHECK(role IN ('user', 'assistant', 'tool', 'system')),
			parts TEXT NOT NULL,
			model TEXT,
			finished_at INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			content TEXT NOT NULL,
			version TEXT NOT NULL,
			UNIQUE(path, session_id, version),
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
		CREATE INDEX IF NOT EXISTS idx_files_session_id ON files(session_id);
	`)

	return err
}

// Close closes the database connection
func (c *Connection) Close() error {
	return c.db.Close()
}

// DB returns the underlying sql.DB
func (c *Connection) DB() *sql.DB {
	return c.db
}

// goose integration placeholder
func init() {
	goose.SetBaseFS(nil)
}
