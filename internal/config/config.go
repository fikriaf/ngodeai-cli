package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for NgodeAI
type Config struct {
	DataDir    string            `json:"dataDir"`
	WorkingDir string            `json:"workingDir"`
	Debug      bool              `json:"debug"`
	Providers  map[string]Provider `json:"providers"`
	AutoCompact bool             `json:"autoCompact"`
}

// Provider holds LLM provider configuration
type Provider struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"`
	Model   string `json:"model,omitempty"`
}

// Load reads configuration from files and environment
func Load(workingDir string, debug bool) (*Config, error) {
	dataDir := getDataDir()

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	cfg := &Config{
		DataDir:     dataDir,
		WorkingDir:  workingDir,
		Debug:       debug,
		Providers:   make(map[string]Provider),
		AutoCompact: true,
	}

	// Load from config file
	if err := cfg.loadFromFile(workingDir); err != nil {
		// Config file is optional
		if debug {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	return cfg, nil
}

func (c *Config) loadFromFile(workingDir string) error {
	// Check local config first
	localPath := filepath.Join(workingDir, ".ngode.json")
	if _, err := os.Stat(localPath); err == nil {
		return c.readFile(localPath)
	}

	// Check global config
	globalPath := filepath.Join(c.DataDir, ".ngode.json")
	if _, err := os.Stat(globalPath); err == nil {
		return c.readFile(globalPath)
	}

	return fmt.Errorf("no config file found")
}

func (c *Config) readFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

func (c *Config) loadFromEnv() {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		c.Providers["openai"] = Provider{APIKey: key}
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		c.Providers["anthropic"] = Provider{APIKey: key}
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		c.Providers["gemini"] = Provider{APIKey: key}
	}
}

func getDataDir() string {
	if dir := os.Getenv("NGODEAI_DATA_DIR"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	return filepath.Join(home, ".ngodeai")
}
