package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"`
	Model   string `json:"model,omitempty"`
}

// AppConfig holds the full config structure
type AppConfig struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// RunWizard runs the interactive setup wizard
func RunWizard() (*AppConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("┌─────────────────────────────────────────────┐")
	fmt.Println("│       🚀 NgodeAI CLI — Setup Wizard         │")
	fmt.Println("└─────────────────────────────────────────────┘")
	fmt.Println()

	// Step 1: Choose provider
	fmt.Println("Pilih AI provider:")
	fmt.Println()
	fmt.Println("  1. Anthropic (Claude 3.5 Sonnet)")
	fmt.Println("  2. OpenAI (GPT-4)")
	fmt.Println("  3. Google Gemini (Gemini 2.0 Flash)")
	fmt.Println("  4. Custom Endpoint (own API server)")
	fmt.Println()

	var provider string
	var config ProviderConfig

	for {
		fmt.Print("Pilihan (1-4): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			provider = "anthropic"
			config = anthropicSetup(reader)
		case "2":
			provider = "openai"
			config = openaiSetup(reader)
		case "3":
			provider = "gemini"
			config = geminiSetup(reader)
		case "4":
			provider = "custom"
			config = customSetup(reader)
		default:
			fmt.Println("❌ Pilihan tidak valid, coba lagi (1-4)")
			continue
		}
		break
	}

	// Confirm
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────┐")
	fmt.Println("│            📋 Konfigurasi                   │")
	fmt.Println("├─────────────────────────────────────────────┤")
	fmt.Printf("│  Provider : %-30s │\n", provider)
	fmt.Printf("│  API Key  : %-30s │\n", maskKey(config.APIKey))
	if config.BaseURL != "" {
		fmt.Printf("│  Base URL : %-30s │\n", config.BaseURL)
	}
	if config.Model != "" {
		fmt.Printf("│  Model    : %-30s │\n", config.Model)
	}
	fmt.Println("└─────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Print("Simpan konfigurasi? (Y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm == "n" || confirm == "no" {
		return nil, fmt.Errorf("setup cancelled")
	}

	// Save config
	appCfg := &AppConfig{
		Providers: map[string]ProviderConfig{
			provider: config,
		},
	}

	if err := saveConfig(appCfg); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ Konfigurasi berhasil disimpan!")
	fmt.Println("🚀 Ketik 'ngodeai' untuk memulai!")
	fmt.Println()

	return appCfg, nil
}

func anthropicSetup(reader *bufio.Reader) ProviderConfig {
	fmt.Println()
	fmt.Println("🔑 Anthropic Claude Setup")
	fmt.Println("   Dapatkan API key: https://console.anthropic.com/settings/keys")
	fmt.Println()
	fmt.Print("   API Key (sk-ant-...): ")
	key, _ := reader.ReadString('\n')
	return ProviderConfig{APIKey: strings.TrimSpace(key)}
}

func openaiSetup(reader *bufio.Reader) ProviderConfig {
	fmt.Println()
	fmt.Println("🔑 OpenAI Setup")
	fmt.Println("   Dapatkan API key: https://platform.openai.com/api-keys")
	fmt.Println()
	fmt.Print("   API Key (sk-...): ")
	key, _ := reader.ReadString('\n')
	return ProviderConfig{APIKey: strings.TrimSpace(key)}
}

func geminiSetup(reader *bufio.Reader) ProviderConfig {
	fmt.Println()
	fmt.Println("🔑 Google Gemini Setup")
	fmt.Println("   Dapatkan API key: https://aistudio.google.com/apikey")
	fmt.Println()
	fmt.Print("   API Key: ")
	key, _ := reader.ReadString('\n')
	return ProviderConfig{APIKey: strings.TrimSpace(key)}
}

func customSetup(reader *bufio.Reader) ProviderConfig {
	fmt.Println()
	fmt.Println("🔧 Custom Endpoint Setup")
	fmt.Println("   Untuk OpenAI-compatible API (LM Studio, Ollama, vLLM, dll)")
	fmt.Println()

	fmt.Print("   Base URL (e.g., http://localhost:11434/v1): ")
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	// Validate and fix URL format
	if strings.Contains(baseURL, "::") {
		baseURL = strings.ReplaceAll(baseURL, "::", "://")
		fmt.Printf("   Fixed URL to: %s\n", baseURL)
	}

	fmt.Print("   Model name (e.g., llama3, qwen2.5): ")
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)

	fmt.Print("   API Key (optional, press Enter to skip): ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)
	if key == "" {
		key = "not-needed"
	}

	return ProviderConfig{
		APIKey:  key,
		BaseURL: baseURL,
		Model:   model,
	}
}

func maskKey(key string) string {
	if len(key) < 10 {
		return "****"
	}
	return key[:6] + strings.Repeat("*", len(key)-10) + key[len(key)-4:]
}

func saveConfig(cfg *AppConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".ngodeai")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, ".ngode.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ConfigExists checks if config already exists
func ConfigExists() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check global config
	globalPath := filepath.Join(home, ".ngodeai", ".ngode.json")
	if _, err := os.Stat(globalPath); err == nil {
		return true
	}

	// Check local config
	localPath := ".ngode.json"
	if _, err := os.Stat(localPath); err == nil {
		return true
	}

	// Check env vars
	if os.Getenv("ANTHROPIC_API_KEY") != "" ||
		os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("GEMINI_API_KEY") != "" {
		return true
	}

	return false
}
