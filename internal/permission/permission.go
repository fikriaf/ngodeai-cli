package permission

import (
	"fmt"
	"strings"
	"sync"
)

// Decision represents the outcome of a permission request
type Decision int

const (
	Deny Decision = iota
	AllowOnce
	AllowSession
)

// Request describes an action that needs permission
type Request struct {
	// Tool is the tool name requesting permission (e.g. "bash", "edit", "write")
	Tool string
	// Description is a human-readable description of the action
	Description string
	// Command is the raw command or operation (for bash, the shell command)
	Command string
}

// callback is the UI hook that prompts the user.
// It must block until the user makes a decision.
type callback func(req Request) Decision

// Service manages permission requests with session-level tracking
type Service struct {
	mu          sync.Mutex
	approved    map[string]bool // session-level approvals (by signature)
	autoApprove map[string]bool // tools that are always safe
	promptFn    callback        // UI callback for user prompts
}

// NewService creates a new permission service.
// promptFn is called when user approval is needed; it must block until decided.
func NewService(promptFn callback) *Service {
	return &Service{
		approved: make(map[string]bool),
		autoApprove: map[string]bool{
			"view": true,
			"glob": true,
			"grep": true,
		},
		promptFn: promptFn,
	}
}

// RequestPermission blocks until the action is approved or denied.
// Returns true if allowed, false if denied.
// Safe read-only tools (view, glob, grep) are auto-approved.
// Previously session-approved signatures are auto-approved.
func (s *Service) RequestPermission(req Request) (bool, error) {
	// Auto-approve safe read-only tools
	if s.isAutoApproved(req.Tool) {
		return true, nil
	}

	// Check session-level approval
	sig := s.signature(req)
	s.mu.Lock()
	if s.approved[sig] {
		s.mu.Unlock()
		return true, nil
	}
	s.mu.Unlock()

	// Ask the user
	if s.promptFn == nil {
		return false, fmt.Errorf("no permission prompt configured")
	}

	decision := s.promptFn(req)

	switch decision {
	case AllowSession:
		s.mu.Lock()
		s.approved[sig] = true
		s.mu.Unlock()
		return true, nil
	case AllowOnce:
		return true, nil
	case Deny:
		return false, nil
	default:
		return false, fmt.Errorf("unknown decision: %d", decision)
	}
}

// isAutoApproved checks if a tool is in the safe list
func (s *Service) isAutoApproved(tool string) bool {
	return s.autoApprove[strings.ToLower(tool)]
}

// signature generates a session-level approval key for a request.
// For bash commands, it uses the first word (the command name) for broad approval.
// For file tools, it uses tool + target path.
func (s *Service) signature(req Request) string {
	if req.Tool == "bash" {
		// Approve by the base command (e.g., "bash:go", "bash:npm")
		parts := strings.Fields(req.Command)
		if len(parts) > 0 {
			return "bash:" + parts[0]
		}
		return "bash:" + req.Command
	}
	return req.Tool + ":" + req.Description
}

// ResetSession clears all session-level approvals
func (s *Service) ResetSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approved = make(map[string]bool)
}

// AddAutoApprove adds a tool to the auto-approve list
func (s *Service) AddAutoApprove(tool string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoApprove[strings.ToLower(tool)] = true
}

// IsApproved checks if a specific signature is already session-approved
func (s *Service) IsApproved(req Request) bool {
	sig := s.signature(req)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.approved[sig]
}
