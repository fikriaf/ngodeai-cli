# NgodeAI CLI

<p align="center">
  <img src="https://raw.githubusercontent.com/fikriaf/ngodeai-cli/main/docs/logo.png" alt="NgodeAI" width="120" />
</p>

<p align="center">
  <strong>Open-source terminal AI coding assistant built with Go</strong>
</p>

<p align="center">
  <a href="https://github.com/fikriaf/ngodeai-cli/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/fikriaf/ngodeai-cli" alt="License" />
  </a>
  <a href="https://github.com/fikriaf/ngodeai-cli/releases">
    <img src="https://img.shields.io/github/v/release/fikriaf/ngodeai-cli" alt="Release" />
  </a>
  <a href="https://github.com/fikriaf/ngodeai-cli/actions/workflows/release.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/fikriaf/ngodeai-cli/release.yml" alt="Build" />
  </a>
  <a href="https://goreportcard.com/report/github.com/fikriaf/ngodeai-cli">
    <img src="https://goreportcard.com/badge/github.com/fikriaf/ngodeai-cli" alt="Go Report Card" />
  </a>
</p>

---

## Screenshots

<p align="center">
  <!-- Replace with actual screenshots -->
  <img src="https://raw.githubusercontent.com/fikriaf/ngodeai-cli/main/docs/screenshot-main.png" alt="Main Interface" width="100%" />
  <em>Interactive TUI with session management</em>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/fikriaf/ngodeai-cli/main/docs/screenshot-tools.png" alt="Tool Execution" width="100%" />
  <em>AI-powered tool execution and code editing</em>
</p>

---

## Overview

NgodeAI is a powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal. Built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework.

## Features

### Core

- 🖥️ **Interactive TUI** — Beautiful terminal interface with keyboard-driven navigation
- 🤖 **Multiple AI Providers** — OpenAI, Anthropic Claude, Google Gemini, and more
- 🛠️ **Tool Integration** — Execute shell commands, search files, edit code, and write files
- 💾 **Session Management** — Save, browse, and switch between conversation sessions
- 🔍 **LSP Integration** — Language Server Protocol support for code intelligence
- 📊 **Cost Tracking** — Monitor token usage and API costs per session

### Developer Experience

- 🔒 **Permission System** — Explicit approval before executing actions
- 📝 **Non-interactive Mode** — Use from scripts with `-p` flag
- ⚡ **Fast Startup** — Single binary, zero dependencies
- 🎨 **Themeable** — Customizable colors and styling
- 🔄 **Auto-compaction** — Automatic context window management for long conversations
- 📋 **Session Browser** — Browse and switch sessions with `/` filter and keyboard navigation

### Tools

| Tool | Description |
|------|-------------|
| `bash` | Execute shell commands |
| `view` | Read file contents |
| `edit` | Edit files with find-and-replace |
| `write` | Create or overwrite files |
| `grep` | Search file contents with regex |
| `glob` | Find files by pattern |

---

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/fikriaf/ngodeai-cli/main/install | bash
```

The install script will:
- Detect your OS (Linux/macOS) and architecture (amd64/arm64)
- Download the latest release from GitHub
- Install to `/usr/local/bin` or `~/.local/bin`
- Verify the installation

### From Releases

Download the appropriate binary from the [releases page](https://github.com/fikriaf/ngodeai-cli/releases):

| Platform | Architecture | File |
|----------|--------------|------|
| Linux | amd64 | `ngodeai_*_linux_amd64.tar.gz` |
| Linux | arm64 | `ngodeai_*_linux_arm64.tar.gz` |
| macOS | amd64 (Intel) | `ngodeai_*_darwin_amd64.tar.gz` |
| macOS | arm64 (Apple Silicon) | `ngodeai_*_darwin_arm64.tar.gz` |
| Windows | amd64 | `ngodeai_*_windows_amd64.zip` |
| Windows | arm64 | `ngodeai_*_windows_arm64.zip` |

```bash
# Example: Linux amd64
curl -LO https://github.com/fikriaf/ngodeai-cli/releases/latest/download/ngodeai_*_linux_amd64.tar.gz
tar -xzf ngodeai_*_linux_amd64.tar.gz
sudo mv ngodeai /usr/local/bin/
```

### From Source

Requires Go 1.24 or later.

```bash
# Install directly
go install github.com/fikriaf/ngodeai-cli@latest

# Or clone and build
git clone https://github.com/fikriaf/ngodeai-cli.git
cd ngodeai-cli
go build -o ngodeai
sudo mv ngodeai /usr/local/bin/
```

### Package Managers

```bash
# Homebrew (coming soon)
brew install fikriaf/tap/ngodeai

# Nix (coming soon)
nix-shell -p ngodeai
```

---

## Quick Start

### Interactive Mode

```bash
# Start the TUI
ngodeai

# Start in a specific directory
ngodeai -c /path/to/project
```

### Non-interactive Mode

```bash
# Single prompt
ngodeai -p "Explain this codebase"

# Pipe input
cat error.log | ngodeai -p "What's wrong with this log?"
```

### Session Management

Once in the TUI:
- Press `/` to filter sessions
- Use `↑/↓` to navigate
- Press `Enter` to select a session
- Press `Esc` to close the session browser

---

## Configuration

NgodeAI looks for configuration in:

1. `./.ngode.json` (local directory)
2. `~/.ngodeai/.ngode.json` (global config)

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENAI_API_KEY` | OpenAI API key | One provider required |
| `ANTHROPIC_API_KEY` | Anthropic Claude API key | One provider required |
| `GEMINI_API_KEY` | Google Gemini API key | One provider required |

### Example Configuration

```json
{
  "providers": {
    "anthropic": {
      "apiKey": "sk-ant-...",
      "model": "claude-3-sonnet-20240229"
    },
    "openai": {
      "apiKey": "sk-...",
      "model": "gpt-4-turbo-preview"
    }
  },
  "autoCompact": true,
  "maxTokens": 4096
}
```

---

## Architecture

NgodeAI follows a clean, modular architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                         TUI Layer                           │
│              (Bubble Tea + Lipgloss)                        │
├─────────────────────────────────────────────────────────────┤
│                      Application                            │
│         (App Container · Session · Message)                 │
├─────────────────────────────────────────────────────────────┤
│                         Agent                               │
│        (LLM Provider · Tool System · Pub/Sub)               │
├─────────────────────────────────────────────────────────────┤
│                        Storage                              │
│                   (SQLite + Goose)                          │
└─────────────────────────────────────────────────────────────┘
```

- **Event-Driven** — Pub/Sub system for decoupled communication
- **Single Binary** — Zero dependencies, easy distribution
- **SQLite Database** — Persistent session and message storage
- **Tool System** — Extensible tool interface for AI actions
- **Provider Abstraction** — Support for multiple LLM providers

See [architecture.md](https://github.com/fikriaf/NgodeAI/blob/main/architecture.md) for detailed documentation.

---

## Development

### Prerequisites

- Go 1.24 or later
- SQLite3

### Building from Source

```bash
git clone https://github.com/fikriaf/ngodeai-cli.git
cd ngodeai-cli
go build -o ngodeai
```

### Running Tests

```bash
go test ./...
```

### Creating a Release

Releases are automated via [GoReleaser](https://goreleaser.com) and GitHub Actions:

```bash
# Local snapshot build
goreleaser release --snapshot --clean

# Trigger a release
git tag v0.3.0
git push origin v0.3.0
```

---

## Roadmap

- [x] Core CLI structure
- [x] Database layer (SQLite)
- [x] Session management
- [x] Message system
- [x] Interactive TUI (Bubble Tea)
- [x] LLM provider integration (OpenAI, Anthropic, Gemini)
- [x] Tool system implementation
- [x] Session browser component
- [x] Streaming responses
- [x] Theme system
- [x] Auto-compaction
- [x] LSP integration
- [x] MCP protocol support
- [x] Model/theme/file pickers
- [ ] Multi-file editing
- [ ] Git integration

---

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Inspired by:
- [Crush](https://github.com/charmbracelet/crush) by Charmbracelet
- [OpenCode](https://github.com/opencode-ai/opencode) by SST/Anomaly
- [Claude Code](https://claude.ai) by Anthropic

---

<p align="center">
  Built with ❤️ by <a href="https://github.com/fikriaf">fikriaf</a>
</p>
