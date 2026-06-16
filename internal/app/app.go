package app

import (
	"context"

	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/message"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// App is the main application container
type App struct {
	Sessions *session.Service
	Messages *message.Service
	Config   *config.Config
}

// New creates a new App instance
func New(ctx context.Context, conn *db.Connection, cfg *config.Config) (*App, error) {
	db := conn.DB()

	return &App{
		Sessions: session.NewService(db),
		Messages: message.NewService(db),
		Config:   cfg,
	}, nil
}
