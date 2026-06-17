package config

import (
	"os"
	"testing"
)

func TestLoadEnvVars(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "test-key-123")
	defer os.Unsetenv("OPENAI_API_KEY")
	
	cfg, err := Load(".", false)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if pCfg, ok := cfg.Providers["openai"]; !ok || pCfg.APIKey != "test-key-123" {
		t.Error("API key not loaded from environment variable")
	}
}
