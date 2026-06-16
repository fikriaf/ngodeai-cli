package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/llm/tools"
)

// MCPTool wraps an MCP server tool as a BaseTool
type MCPTool struct {
	client      *Client
	tool        Tool
	serverName  string
}

// NewMCPTool creates a new MCP tool wrapper
func NewMCPTool(client *Client, tool Tool) *MCPTool {
	return &MCPTool{
		client:     client,
		tool:       tool,
		serverName: client.Name(),
	}
}

// Info returns the tool metadata for the LLM
func (t *MCPTool) Info() tools.ToolInfo {
	// Build parameters from MCP tool schema
	params := t.tool.InputSchema
	if params == nil {
		params = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	// Extract required fields
	var required []string
	if req, ok := params["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	// Build a descriptive name including server name
	name := fmt.Sprintf("mcp_%s_%s", t.serverName, t.tool.Name)
	// Sanitize name to be a valid identifier
	name = sanitizeToolName(name)

	description := t.tool.Description
	if description == "" {
		description = fmt.Sprintf("MCP tool from %s server", t.serverName)
	}

	// Add server context to description
	description = fmt.Sprintf("[%s] %s", t.serverName, description)

	return tools.ToolInfo{
		Name:        name,
		Description: description,
		Parameters:  params,
		Required:    required,
	}
}

// Run executes the MCP tool
func (t *MCPTool) Run(ctx context.Context, params tools.ToolCall) (tools.ToolResponse, error) {
	// Convert args to map[string]any
	args := make(map[string]any)
	for k, v := range params.Args {
		args[k] = v
	}

	// Execute the MCP tool
	result, err := t.client.ExecuteTool(ctx, t.tool.Name, args)
	if err != nil {
		return tools.ToolResponse{
			IsError: true,
			Content: fmt.Sprintf("MCP tool error: %v", err),
		}, nil
	}

	// Convert result to string
	var content strings.Builder
	for _, block := range result.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		} else {
			// For non-text content, serialize as JSON
			data, _ := json.Marshal(block)
			content.WriteString(string(data))
		}
	}

	return tools.ToolResponse{
		IsError: result.IsError,
		Content: content.String(),
	}, nil
}

// sanitizeToolName converts a name to a valid tool identifier
func sanitizeToolName(name string) string {
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		} else if r == '-' || r == '.' || r == '/' {
			result.WriteRune('_')
		}
	}
	return result.String()
}

// ── Service ─────────────────────────────────────────────────────────────────

// Service manages MCP servers and their tools
type Service struct {
	servers map[string]*Client
	tools   []tools.BaseTool
}

// NewService creates a new MCP service
func NewService() *Service {
	return &Service{
		servers: make(map[string]*Client),
		tools:   make([]tools.BaseTool, 0),
	}
}

// AddServer adds and connects to an MCP server
func (s *Service) AddServer(ctx context.Context, name string, config ServerConfig) error {
	client := NewClient(name, config)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	s.servers[name] = client

	// Discover tools from this server
	mcpTools, err := client.DiscoverTools(ctx)
	if err != nil {
		client.Close()
		delete(s.servers, name)
		return fmt.Errorf("failed to discover tools from %s: %w", name, err)
	}

	// Wrap each tool
	for _, tool := range mcpTools {
		wrapped := NewMCPTool(client, tool)
		s.tools = append(s.tools, wrapped)
	}

	return nil
}

// GetTools returns all discovered MCP tools
func (s *Service) GetTools() []tools.BaseTool {
	return s.tools
}

// Close shuts down all MCP server connections
func (s *Service) Close() {
	for _, client := range s.servers {
		client.Close()
	}
	s.servers = make(map[string]*Client)
	s.tools = nil
}

// HasServers returns true if there are any connected servers
func (s *Service) HasServers() bool {
	return len(s.servers) > 0
}

// ServerCount returns the number of connected servers
func (s *Service) ServerCount() int {
	return len(s.servers)
}

// ToolCount returns the number of discovered tools
func (s *Service) ToolCount() int {
	return len(s.tools)
}
