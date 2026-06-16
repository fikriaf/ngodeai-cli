package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fikriaf/ngodeai-cli/internal/lsp"
)

// DiagnosticsTool provides access to LSP diagnostics as a tool
type DiagnosticsTool struct {
	client *lsp.Client
}

// NewDiagnosticsTool creates a new diagnostics tool with an LSP client
func NewDiagnosticsTool(client *lsp.Client) *DiagnosticsTool {
	return &DiagnosticsTool{
		client: client,
	}
}

// Info returns the tool metadata for the LLM
func (d *DiagnosticsTool) Info() ToolInfo {
	return ToolInfo{
		Name:        "diagnostics",
		Description: "Get diagnostic information (errors, warnings) for files. Returns compilation errors and linting issues.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"description": "Diagnostic mode: 'file' for specific file or 'project' for entire project",
					"enum":        []string{"file", "project"},
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File path (required when mode is 'file')",
				},
			},
			"required": []string{"mode"},
		},
		Required: []string{"mode"},
	}
}

// Run executes the diagnostics tool
func (d *DiagnosticsTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	mode, _ := params.Args["mode"].(string)
	path, _ := params.Args["path"].(string)

	if mode == "" {
		return ToolResponse{IsError: true, Content: "mode parameter is required (must be 'file' or 'project')"}, nil
	}

	switch mode {
	case "file":
		if path == "" {
			return ToolResponse{IsError: true, Content: "path parameter is required for file mode"}, nil
		}
		return d.getDiagnosticsForFile(path)

	case "project":
		return d.getDiagnosticsForProject()

	default:
		return ToolResponse{IsError: true, Content: fmt.Sprintf("unknown mode: %s (must be 'file' or 'project')", mode)}, nil
	}
}

// getDiagnosticsForFile gets diagnostics for a single file
func (d *DiagnosticsTool) getDiagnosticsForFile(filePath string) (ToolResponse, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	uri := "file://" + absPath
	ext := strings.ToLower(filepath.Ext(filePath))
	languageID := extensionToLanguage(ext)

	diagnostics := d.client.GetDiagnostics(uri)

	return formatFileDiagnostics(diagnostics, filePath, languageID), nil
}

// getDiagnosticsForProject gets diagnostics for all open files in the project
func (d *DiagnosticsTool) getDiagnosticsForProject() (ToolResponse, error) {
	allDiagnostics := d.client.GetAllDiagnostics()

	if len(allDiagnostics) == 0 {
		return ToolResponse{
			Content: "No active diagnostics available. Open a file first to enable diagnostics.\n\nSupported languages:\n- Go (gopls)\n- TypeScript/JavaScript (typescript-language-server)",
		}, nil
	}

	var output strings.Builder
	output.WriteString("=== Project Diagnostics ===\n\n")

	fileCount := 0
	totalErrors := 0
	totalWarnings := 0

	for uri, diags := range allDiagnostics {
		if len(diags) == 0 {
			continue
		}

		fileCount++
		errorCount := 0
		warningCount := 0

		for _, diag := range diags {
			switch diag.Severity {
			case lsp.DiagnosticError:
				totalErrors++
				errorCount++
			case lsp.DiagnosticWarning:
				totalWarnings++
				warningCount++
			}
		}

		output.WriteString(fmt.Sprintf("[%s]\n", trimURI(uri)))
		output.WriteString(fmt.Sprintf("  Errors: %d, Warnings: %d, Total: %d\n\n", errorCount, warningCount, len(diags)))

		for i, diag := range diags {
			line := diag.Range.Start.Line + 1
			col := diag.Range.Start.Character + 1

			severityChar := "?"
			switch diag.Severity {
			case lsp.DiagnosticError:
				severityChar = "E"
			case lsp.DiagnosticWarning:
				severityChar = "W"
			case lsp.DiagnosticInfo:
				severityChar = "I"
			case lsp.DiagnosticHint:
				severityChar = "H"
			}

			output.WriteString(fmt.Sprintf("  [%d] [%s] Line %d:%d: %s\n", i+1, severityChar, line, col, diag.Message))
		}

		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("=== Summary ===\n"))
	output.WriteString(fmt.Sprintf("Files with diagnostics: %d\n", fileCount))
	output.WriteString(fmt.Sprintf("Total errors: %d\n", totalErrors))
	output.WriteString(fmt.Sprintf("Total warnings: %d\n", totalWarnings))

	return ToolResponse{Content: output.String()}, nil
}

// formatFileDiagnostics formats diagnostics for display
func formatFileDiagnostics(diagnostics []lsp.Diagnostic, title, languageID string) ToolResponse {
	if len(diagnostics) == 0 {
		return ToolResponse{
			Content: fmt.Sprintf("✓ No issues found in %s\n\nLanguage: %s", title, languageID),
		}
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("=== Diagnostics for %s ===\n\n", title))

	errorCount := 0
	warningCount := 0

	for _, diag := range diagnostics {
		switch diag.Severity {
		case lsp.DiagnosticError:
			errorCount++
		case lsp.DiagnosticWarning:
			warningCount++
		}
	}

	output.WriteString(fmt.Sprintf("Found %d issue(s):\n", len(diagnostics)))
	output.WriteString(fmt.Sprintf("  Errors: %d\n", errorCount))
	output.WriteString(fmt.Sprintf("  Warnings: %d\n", warningCount))
	output.WriteString("\n")

	for i, diag := range diagnostics {
		line := diag.Range.Start.Line + 1
		col := diag.Range.Start.Character + 1
		endLine := diag.Range.End.Line + 1
		endCol := diag.Range.End.Character + 1

		severityEmoji := "⚠️"
		severityWord := "Warning"
		if diag.Severity == lsp.DiagnosticError {
			severityEmoji = "❌"
			severityWord = "Error"
		}

		output.WriteString(fmt.Sprintf("\n[%d] %s %s\n", i+1, severityEmoji, severityWord))
		output.WriteString(fmt.Sprintf("  Location: Line %d:%d - Line %d:%d\n", line, col, endLine, endCol))
		output.WriteString(fmt.Sprintf("  Message: %s\n", diag.Message))
		if diag.Code != "" {
			output.WriteString(fmt.Sprintf("  Code: %s\n", diag.Code))
		}
		if diag.Source != "" {
			output.WriteString(fmt.Sprintf("  Source: %s\n", diag.Source))
		}
	}

	return ToolResponse{Content: output.String()}
}

// extensionToLanguage maps file extensions to LSP language IDs
func extensionToLanguage(ext string) string {
	mapping := map[string]string{
		".go":       "go",
		".ts":       "typescript",
		".tsx":      "typescriptreact",
		".js":       "javascript",
		".jsx":      "javascriptreact",
		".py":       "python",
		".rs":       "rust",
		".java":     "java",
		".c":        "c",
		".cpp":      "cpp",
		".h":        "c-header",
		".hpp":      "cpp-header",
		".swift":    "swift",
		".php":      "php",
		".rb":       "ruby",
		".r":        "r",
		".scala":    "scala",
		".kt":       "kotlin",
		".dart":     "dart",
		".html":     "html",
		".css":      "css",
		".scss":     "scss",
		".sass":     "sass",
		".less":     "less",
		".json":     "json",
		".xml":      "xml",
		".yaml":     "yaml",
		".yml":      "yaml",
		".markdown": "markdown",
		".md":       "markdown",
		".sh":       "shellscript",
		".zsh":      "shellscript",
		".bash":     "shellscript",
		".sql":      "sql",
		".graphql":  "graphql",
		".proto":    "proto",
		".toml":     "toml",
		".conf":     "ini",
		".env":      "ini",
		".vue":      "vue",
	}

	id, ok := mapping[ext]
	if !ok {
		return "plaintext"
	}
	return id
}

// trimURI removes the file:// prefix from URIs
func trimURI(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return uri[7:]
	}
	return uri
}
