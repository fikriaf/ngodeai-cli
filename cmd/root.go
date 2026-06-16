package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fikriaf/ngodeai-cli/internal/app"
	"github.com/fikriaf/ngodeai-cli/internal/config"
	"github.com/fikriaf/ngodeai-cli/internal/db"
	"github.com/fikriaf/ngodeai-cli/internal/logging"
	"github.com/spf13/cobra"
)

var (
	cwd    string
	prompt string
	debug  bool
	version string = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "ngodeai",
	Short: "NgodeAI - Terminal AI coding assistant",
	Long:  "NgodeAI is an open-source terminal AI coding assistant built with Go.",
	RunE:  run,
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

	// Non-interactive mode
	if prompt != "" {
		return runNonInteractive(ctx, a, prompt)
	}

	// Interactive mode - TODO: launch TUI
	fmt.Println("NgodeAI CLI v" + version)
	fmt.Println("Interactive mode not yet implemented. Use -p flag for non-interactive mode.")
	return nil
}

func runNonInteractive(ctx context.Context, a *app.App, prompt string) error {
	// TODO: implement non-interactive execution
	fmt.Printf("Prompt: %s\n", prompt)
	fmt.Println("Non-interactive mode not yet implemented.")
	return nil
}
