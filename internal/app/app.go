package app

import (
	"context"
	"fmt"
	"os"

	"github.com/fikriaf/ngodeai-cli/internal/commands"
	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/llm/agent"
	"github.com/fikriaf/ngodeai-cli/internal/llm/provider"
	"github.com/fikriaf/ngodeai-cli/internal/llm/tools"
	"github.com/fikriaf/ngodeai-cli/internal/mcp"
	"github.com/fikriaf/ngodeai-cli/internal/message"
	"github.com/fikriaf/ngodeai-cli/internal/session"
)

// App is the main application container
type App struct {
	Sessions    *session.Service
	Messages    *message.Service
	Config      *config.Config
	Agent       *agent.Agent
	Commands    *commands.Service
	MCPService  *mcp.Service
}

// New creates a new App instance
func New(ctx context.Context, conn *db.Connection, cfg *config.Config) (*App, error) {
	database := conn.DB()

	sessions := session.NewService(database)
	messages := message.NewService(database)

	// Initialize custom commands service
	cmdSvc := commands.NewService(cfg.WorkingDir)
	if err := cmdSvc.Load(); err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "Warning: failed to load custom commands: %v\n", err)
		}
	}

	// Initialize MCP service
	mcpSvc := mcp.NewService()

	// Connect to configured MCP servers
	for name, serverCfg := range cfg.MCPServers {
		mcpCfg := mcp.ServerConfig{
			Command: serverCfg.Command,
			Args:    serverCfg.Args,
			Env:     serverCfg.Env,
		}
		if err := mcpSvc.AddServer(ctx, name, mcpCfg); err != nil {
			if cfg.Debug {
				fmt.Fprintf(os.Stderr, "Warning: failed to connect to MCP server %s: %v\n", name, err)
			}
		} else if cfg.Debug {
			fmt.Fprintf(os.Stderr, "Connected to MCP server: %s\n", name)
		}
	}

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

	// Add MCP tools to the tool list
	if mcpSvc.HasServers() {
		mcpTools := mcpSvc.GetTools()
		toolList = append(toolList, mcpTools...)
	}

	// Check for custom provider first, then Anthropic, OpenAI, Gemini
	if cp, ok := cfg.Providers["custom"]; ok && cp.APIKey != "" {
		p = provider.NewOpenAIWithConfig(cp.APIKey, cp.Model, cp.BaseURL, cp.MaxTokens, cp.Timeout, cp.ContextWindow)
	} else if ap, ok := cfg.Providers["anthropic"]; ok && ap.APIKey != "" {
		p = provider.NewAnthropic(ap.APIKey, ap.Model)
	} else if op, ok := cfg.Providers["openai"]; ok && op.APIKey != "" {
		p = provider.NewOpenAI(op.APIKey, op.Model)
	} else if gp, ok := cfg.Providers["gemini"]; ok && gp.APIKey != "" {
		p = provider.NewGemini(gp.APIKey, gp.Model)
	}

	var ag *agent.Agent
	if p != nil {
		ag = agent.New(p, toolList, sessions, messages)
	}

	return &App{
		Sessions:   sessions,
		Messages:   messages,
		Config:     cfg,
		Agent:      ag,
		Commands:   cmdSvc,
		MCPService: mcpSvc,
	}, nil
}

// Close cleans up resources
func (a *App) Close() {
	if a.MCPService != nil {
		a.MCPService.Close()
	}
}
