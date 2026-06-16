package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// ServerConfig holds configuration for an MCP server
type ServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in MCP responses
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Client manages communication with an MCP server via stdio
type Client struct {
	name       string
	config     ServerConfig
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	reader     *bufio.Reader
	mu         sync.Mutex
	requestID  atomic.Int64
	pending    map[int64]chan *json.RawMessage
	pendingMu  sync.Mutex
	tools      []Tool
	connected  bool
	closeOnce  sync.Once
	closeCh    chan struct{}
}

// jsonRPCRequest represents a JSON-RPC 2.0 request
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response
type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int64            `json:"id"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError    `json:"error,omitempty"`
}

// jsonRPCError represents a JSON-RPC 2.0 error
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewClient creates a new MCP client
func NewClient(name string, config ServerConfig) *Client {
	return &Client{
		name:    name,
		config:  config,
		pending: make(map[int64]chan *json.RawMessage),
		closeCh: make(chan struct{}),
	}
}

// Connect starts the MCP server process and initializes the connection
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// Start the server process
	cmd := exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	if len(c.config.Env) > 0 {
		cmd.Env = c.config.Env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = nil // Could capture for logging if needed

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.reader = bufio.NewReader(stdout)

	// Start reading responses in background
	go c.readResponses()

	// Initialize the server
	if err := c.initialize(ctx); err != nil {
		c.cleanup()
		return fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	c.connected = true
	return nil
}

// initialize sends the initialize request to the MCP server
func (c *Client) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "ngodeai-cli",
			"version": "0.2.0",
		},
	}

	_, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	// Send initialized notification
	return c.sendNotification("notifications/initialized", nil)
}

// DiscoverTools queries the server for available tools
func (c *Client) DiscoverTools(ctx context.Context) ([]Tool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to MCP server")
	}

	result, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(*result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	c.tools = response.Tools
	return c.tools, nil
}

// ExecuteTool calls an MCP tool with the given arguments
func (c *Client) ExecuteTool(ctx context.Context, toolName string, args map[string]any) (*ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected to MCP server")
	}

	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}

	result, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var toolResult ToolResult
	if err := json.Unmarshal(*result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &toolResult, nil
}

// Close shuts down the MCP server connection
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		c.cleanup()
	})
	return nil
}

// cleanup closes all resources
func (c *Client) cleanup() {
	c.connected = false
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
}

// sendRequest sends a JSON-RPC request and waits for a response
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	id := c.requestID.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Create response channel
	respCh := make(chan *json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// Send the request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	select {
	case result := <-respCh:
		if result == nil {
			return nil, fmt.Errorf("connection closed")
		}
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	case <-c.closeCh:
		return nil, fmt.Errorf("connection closed")
	}
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (c *Client) sendNotification(method string, params interface{}) error {
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

// readResponses continuously reads and processes server responses
func (c *Client) readResponses() {
	for {
		select {
		case <-c.closeCh:
			return
		default:
		}

		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				// Connection error - clean up pending requests
				c.pendingMu.Lock()
				for _, ch := range c.pending {
					close(ch)
				}
				c.pending = make(map[int64]chan *json.RawMessage)
				c.pendingMu.Unlock()
			}
			return
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // Skip malformed responses
		}

		// Route response to pending request
		c.pendingMu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			if resp.Error != nil {
				// Send nil to indicate error (caller should handle)
				ch <- nil
			} else {
				ch <- resp.Result
			}
		}
		c.pendingMu.Unlock()
	}
}

// Name returns the server name
func (c *Client) Name() string {
	return c.name
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	return c.connected
}
