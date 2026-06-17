package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fikriaf/ngodeai-cli/internal/app"
	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/logging"
	"github.com/fikriaf/ngodeai-cli/internal/setup"
	"github.com/fikriaf/ngodeai-cli/internal/tui"
	"github.com/spf13/cobra"
)

var (
	cwd     string
	prompt  string
	debug   bool
	version string = "0.5.0"
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

	// Add setup subcommand
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Run interactive setup wizard",
		Long:  "Configure your AI provider (Anthropic, OpenAI, Gemini, or custom endpoint)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := setup.RunWizard()
			return err
		},
	}
	rootCmd.AddCommand(setupCmd)
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
	defer a.Close()

	// Check if provider is configured - if not, run setup wizard
	if a.Agent == nil {
		fmt.Println("⚠️  No LLM provider configured!")
		fmt.Println()
		fmt.Print("Run setup wizard? (Y/n): ")
		
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		
		if response == "n" || response == "no" {
			fmt.Println("Setup cancelled. You can run 'ngodeai setup' later.")
			return nil
		}
		
		// Run interactive setup wizard
		_, err = setup.RunWizard()
		if err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}
		
		// Reload configuration after setup
		cfg, err = config.Load(absCwd, debug)
		if err != nil {
			return fmt.Errorf("failed to reload config: %w", err)
		}
		
		// Recreate app with new config
		a, err = app.New(ctx, conn, cfg)
		if err != nil {
			return fmt.Errorf("failed to create app: %w", err)
		}
		defer a.Close()
		
		if a.Agent == nil {
			return fmt.Errorf("setup completed but provider still not configured")
		}
		
		fmt.Println("✅ Setup complete! Starting NgodeAI...")
		fmt.Println()
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
