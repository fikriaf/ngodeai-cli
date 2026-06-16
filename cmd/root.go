package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fikriaf/ngodeai-cli/internal/app"
	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/logging"
	"github.com/fikriaf/ngodeai-cli/internal/tui"
	"github.com/spf13/cobra"
)

var (
	cwd     string
	prompt  string
	debug   bool
	version string = "0.2.0"
)

var rootCmd = &cobra.Command{
	Use:   "ngodeai",
	Short: "NgodeAI - Terminal AI coding assistant",
	Long:  `NgodeAI is an open-source terminal AI coding assistant built with Go.

It provides an interactive TUI for chatting with AI models and executing tools.
Supported providers: OpenAI, Anthropic Claude, Google Gemini.

Set your API key via environment variables:
  export OPENAI_API_KEY=sk-...
  export ANTHROPIC_API_KEY=sk-ant-...`,
	RunE: run,
}

func init() {
	rootCmd.Flags().StringVarP(&cwd, "cwd", "c", ".", "Working directory")
	rootCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Non-interactive prompt")
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	rootCmd.Flags().BoolP("version", "v", false, "Print version")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Print version if requested
	if v, _ := cmd.Flags().GetBool("version"); v {
		fmt.Printf("ngodeai version %s\n", version)
		return nil
	}

	ctx := context.Background()

	// Resolve working directory
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}

	// Load configuration
	logging.Info("Loading configuration", "cwd", absCwd)
	cfg, err := config.Load(absCwd, debug)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect database
	logging.Info("Connecting database")
	conn, err := db.Connect(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	defer conn.Close()

	// Create application
	logging.Info("Creating application")
	a, err := app.New(ctx, conn, cfg)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// Check if provider is configured
	if a.Agent == nil {
		fmt.Println("⚠️  No LLM provider configured!")
		fmt.Println("")
		fmt.Println("Set an API key via environment variables:")
		fmt.Println("  export OPENAI_API_KEY=sk-...")
		fmt.Println("  export ANTHROPIC_API_KEY=sk-ant-...")
		fmt.Println("")
		fmt.Println("Or create a .ngode.json config file:")
		fmt.Println(`  {
    "providers": {
      "openai": { "apiKey": "sk-..." }
    }
  }`)
		return nil
	}

	// Non-interactive mode
	if prompt != "" {
		return runNonInteractive(ctx, a, prompt)
	}

	// Interactive mode - launch TUI
	p := tea.NewProgram(tui.New(a), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func runNonInteractive(ctx context.Context, a *app.App, prompt string) error {
	// Create a session
	sess, err := a.Sessions.Create("CLI Session")
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Run the agent
	response, err := a.Agent.Run(ctx, sess.ID, prompt)
	if err != nil {
		return fmt.Errorf("agent error: %w", err)
	}

	fmt.Println(response)
	return nil
}
