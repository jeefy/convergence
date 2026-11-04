package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundtrip(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cfg := DefaultConfig()
	cfg.Engine = "podman"
	cfg.Provider = ProviderCopilot
	cfg.CopilotToken = "test_token_123"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Engine != "podman" {
		t.Errorf("expected engine podman, got %s", loaded.Engine)
	}
	if loaded.Provider != ProviderCopilot {
		t.Errorf("expected provider copilot, got %s", loaded.Provider)
	}
	if loaded.CopilotToken != "test_token_123" {
		t.Errorf("expected token test_token_123, got %s", loaded.CopilotToken)
	}
	if len(loaded.StandinSalt) != 64 {
		t.Errorf("expected 64 char hex salt, got length %d", len(loaded.StandinSalt))
	}

	path := filepath.Join(tempHome, ".convergence", "config.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected permissions 0600, got %o", perm)
	}
}
