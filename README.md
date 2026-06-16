# NgodeAI CLI

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
</p>

## Overview

NgodeAI is a powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal. Built with Go and the [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework.

## Features

- 🖥️ **Interactive TUI** - Beautiful terminal interface with themes
- 🤖 **Multiple AI Providers** - OpenAI, Anthropic, Google Gemini, and more
- 🛠️ **Tool Integration** - Execute commands, search files, and modify code
- 💾 **Session Management** - Save and manage multiple conversation sessions
- 🔍 **LSP Integration** - Language Server Protocol support for code intelligence
- 📊 **Cost Tracking** - Monitor token usage and API costs
- 🔒 **Permission System** - Explicit approval before executing actions

## Installation

### From Source

```bash
go install github.com/fikriaf/ngodeai-cli@latest
```

### Using the Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/fikriaf/ngodeai-cli/main/install | bash
```

### From Releases

Download the latest release from the [releases page](https://github.com/fikriaf/ngodeai-cli/releases).

## Quick Start

```bash
# Start interactive mode
ngodeai

# Non-interactive mode
ngodeai -p "Explain this codebase"

# Specify working directory
ngodeai -c /path/to/project
```

## Configuration

NgodeAI looks for configuration in:

1. `./.ngode.json` (local directory)
2. `~/.ngodeai/.ngode.json` (global config)

### Environment Variables

- `OPENAI_API_KEY` - OpenAI API key
- `ANTHROPIC_API_KEY` - Anthropic Claude API key
- `GEMINI_API_KEY` - Google Gemini API key

### Example Configuration

```json
{
  "providers": {
    "openai": {
      "apiKey": "sk-...",
      "model": "gpt-4"
    }
  },
  "autoCompact": true
}
```

## Architecture

NgodeAI follows a clean, modular architecture:

- **Event-Driven** - Pub/Sub system for decoupled communication
- **Single Binary** - Zero dependencies, easy distribution
- **SQLite Database** - Persistent session and message storage
- **Tool System** - Extensible tool interface for AI actions
- **Provider Abstraction** - Support for multiple LLM providers

See [architecture.md](https://github.com/fikriaf/NgodeAI/blob/main/architecture.md) for detailed documentation.

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

## Roadmap

- [x] Core CLI structure
- [x] Database layer (SQLite)
- [x] Session management
- [x] Message system
- [ ] Interactive TUI (Bubble Tea)
- [ ] LLM provider integration
- [ ] Tool system implementation
- [ ] LSP integration
- [ ] MCP protocol support
- [ ] Theme system
- [ ] Auto-compaction

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Credits

Inspired by:
- [Crush](https://github.com/charmbracelet/crush) by Charmbracelet
- [OpenCode](https://github.com/opencode-ai/opencode) by SST/Anomaly
- [Claude Code](https://claude.ai) by Anthropic

---

Built with ❤️ by [fikriaf](https://github.com/fikriaf)
