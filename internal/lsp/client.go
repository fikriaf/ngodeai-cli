package lsp

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

// Client is a JSON-RPC client that communicates with LSP servers over stdio
type Client struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
	mu          sync.Mutex
	idCounter   atomic.Int64
	pending     map[int64]chan *json.RawMessage
	diagnostics map[string][]Diagnostic // uri -> diagnostics
	diagMu      sync.RWMutex
	initialized bool
	closed      bool
	closeMu     sync.Mutex
}

// Diagnostic represents an LSP diagnostic
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Source   string `json:"source"`
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
}

// Diagnostic severity constants
const (
	DiagnosticError   = 1
	DiagnosticWarning = 2
	DiagnosticInfo    = 3
	DiagnosticHint    = 4
)

// Range represents a text range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// jsonRPCRequest is a JSON-RPC 2.0 request
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response
type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *int64           `json:"id,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError    `json:"error,omitempty"`
	Method  string           `json:"method,omitempty"` // for notifications
	Params  json.RawMessage  `json:"params,omitempty"` // for notifications
}

// jsonRPCError is a JSON-RPC 2.0 error
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeParams are the parameters for the initialize request
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	ClientInfo   *ClientInfo        `json:"clientInfo,omitempty"`
	RootURI      string             `json:"rootUri"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientInfo contains information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities describes the capabilities the client supports
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities describes text document capabilities
type TextDocumentClientCapabilities struct {
	Synchronization *SynchronizationCapabilities `json:"synchronization,omitempty"`
	PublishDiagnostics *PublishDiagnosticsCapabilities `json:"publishDiagnostics,omitempty"`
}

// SynchronizationCapabilities describes sync capabilities
type SynchronizationCapabilities struct {
	DidChange     bool `json:"didChange"`
	DidOpen       bool `json:"didOpen"`
	DidClose      bool `json:"didClose"`
	DidSave       bool `json:"didSave"`
}

// PublishDiagnosticsCapabilities describes diagnostic capabilities
type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier includes a version
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentItem is a text document item
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpenTextDocumentParams are params for textDocument/didOpen
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams are params for textDocument/didChange
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// TextDocumentContentChangeEvent describes a content change
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"` // Full document content
}

// PublishDiagnosticsParams are params for textDocument/publishDiagnostics
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// NewClient creates a new LSP client
func NewClient() *Client {
	return &Client{
		pending:     make(map[int64]chan *json.RawMessage),
		diagnostics: make(map[string][]Diagnostic),
	}
}

// Start starts the LSP server process
func (c *Client) Start(ctx context.Context, command string, args ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cmd = exec.CommandContext(ctx, command, args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	c.stdout = bufio.NewReader(stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Start reading responses in background
	go c.readLoop()

	return nil
}

// readLoop reads JSON-RPC responses from the LSP server
func (c *Client) readLoop() {
	for {
		msg, err := c.readMessage()
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		c.handleResponse(msg)
	}
}

// readMessage reads a single JSON-RPC message from stdout
func (c *Client) readMessage() (*jsonRPCResponse, error) {
	// Read headers until empty line
	contentLength := 0
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = trimCRLF(line)

		if line == "" {
			break
		}

		if len(line) > 16 && line[:16] == "Content-Length: " {
			fmt.Sscanf(line[16:], "%d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no content length")
	}

	// Read the body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(c.stdout, body)
	if err != nil {
		return nil, err
	}

	var msg jsonRPCResponse
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// handleResponse processes a received JSON-RPC message
func (c *Client) handleResponse(msg *jsonRPCResponse) {
	// Check if it's a notification (no ID)
	if msg.ID == nil && msg.Method != "" {
		c.handleNotification(msg)
		return
	}

	// It's a response to a request
	if msg.ID != nil {
		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.mu.Unlock()

		if ok {
			if msg.Error != nil {
				ch <- nil // Signal error
			} else {
				ch <- msg.Result
			}
			close(ch)
		}
	}
}

// handleNotification processes server-to-client notifications
func (c *Client) handleNotification(msg *jsonRPCResponse) {
	switch msg.Method {
	case "textDocument/publishDiagnostics":
		var params PublishDiagnosticsParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return
		}
		c.diagMu.Lock()
		c.diagnostics[params.URI] = params.Diagnostics
		c.diagMu.Unlock()
	}
}

// sendRequest sends a JSON-RPC request and waits for response
func (c *Client) sendRequest(method string, params interface{}) (*json.RawMessage, error) {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.closeMu.Unlock()

	id := c.idCounter.Add(1)

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create response channel
	ch := make(chan *json.RawMessage, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	// Write the message
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	c.mu.Lock()
	_, err = c.stdin.Write([]byte(header))
	if err == nil {
		_, err = c.stdin.Write(body)
	}
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	select {
	case result := <-ch:
		if result == nil {
			return nil, fmt.Errorf("request failed")
		}
		return result, nil
	case <-time.After(30 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("request timed out")
	}
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (c *Client) sendNotification(method string, params interface{}) error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return fmt.Errorf("client is closed")
	}
	c.closeMu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	return nil
}

// Initialize sends the initialize request to the LSP server
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	params := InitializeParams{
		ProcessID: 0, // 0 means the client and server are the same process (LSP spec)
		ClientInfo: &ClientInfo{
			Name:    "ngodeai-cli",
			Version: "1.0.0",
		},
		RootURI: rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Synchronization: &SynchronizationCapabilities{
					DidChange: true,
					DidOpen:   true,
					DidClose:  true,
					DidSave:   true,
				},
				PublishDiagnostics: &PublishDiagnosticsCapabilities{
					RelatedInformation: true,
				},
			},
		},
	}

	_, err := c.sendRequest("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Send initialized notification
	if err := c.sendNotification("initialized", struct{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	c.initialized = true
	return nil
}

// DidOpen notifies the server that a document was opened
func (c *Client) DidOpen(uri, languageID, text string) error {
	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	}
	return c.sendNotification("textDocument/didOpen", params)
}

// DidChange notifies the server that a document was changed
func (c *Client) DidChange(uri string, version int, text string) error {
	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	}
	return c.sendNotification("textDocument/didChange", params)
}

// GetDiagnostics returns current diagnostics for a file URI
func (c *Client) GetDiagnostics(uri string) []Diagnostic {
	c.diagMu.RLock()
	defer c.diagMu.RUnlock()

	diags, ok := c.diagnostics[uri]
	if !ok {
		return nil
	}

	// Return a copy
	result := make([]Diagnostic, len(diags))
	copy(result, diags)
	return result
}

// GetAllDiagnostics returns diagnostics for all files
func (c *Client) GetAllDiagnostics() map[string][]Diagnostic {
	c.diagMu.RLock()
	defer c.diagMu.RUnlock()

	result := make(map[string][]Diagnostic, len(c.diagnostics))
	for uri, diags := range c.diagnostics {
		copied := make([]Diagnostic, len(diags))
		copy(copied, diags)
		result[uri] = copied
	}
	return result
}

// IsInitialized returns whether the client has been initialized
func (c *Client) IsInitialized() bool {
	return c.initialized
}

// Close shuts down the LSP server connection
func (c *Client) Close() error {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return nil
	}
	c.closed = true
	c.closeMu.Unlock()

	// Send shutdown request
	c.sendRequest("shutdown", nil)
	c.sendNotification("exit", nil)

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	return nil
}

// trimCRLF trims \r\n from a string
func trimCRLF(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\r' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
