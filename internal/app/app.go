package app

import (
	"context"

	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/llm/agent"
	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/llm/tools"
	"github.com/fikriaf/ngodeai-cli/internal/message"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// App is the main application container
type App struct {
	Sessions *session.Service
	Messages *message.Service
	Config   *config.Config
	Agent    *agent.Agent
}

// New creates a new App instance
func New(ctx context.Context, conn *db.Connection, cfg *config.Config) (*App, error) {
	database := conn.DB()

	sessions := session.NewService(database)
	messages := message.NewService(database)

	// Determine which provider to use
	var p provider.Provider
	toolList := []tools.BaseTool{
		tools.NewViewTool(),
		tools.NewBashTool(),
		tools.NewEditTool(),
		tools.NewWriteTool(),
		tools.NewGrepTool(),
		tools.NewGlobTool(),
	}

	// Check for Anthropic first, then OpenAI, then Gemini
	if cfg.Providers["anthropic"].APIKey != "" {
		p = provider.NewAnthropic(cfg.Providers["anthropic"].APIKey, cfg.Providers["anthropic"].Model)
	} else if cfg.Providers["openai"].APIKey != "" {
		p = provider.NewOpenAI(cfg.Providers["openai"].APIKey, cfg.Providers["openai"].Model)
	} else if cfg.Providers["gemini"].APIKey != "" {
		p = provider.NewGemini(cfg.Providers["gemini"].APIKey, cfg.Providers["gemini"].Model)
	}

	var ag *agent.Agent
	if p != nil {
		ag = agent.New(p, toolList, sessions, messages)
	}

	return &App{
		Sessions: sessions,
		Messages: messages,
		Config:   cfg,
		Agent:    ag,
	}, nil
}
