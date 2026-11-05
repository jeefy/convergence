package clientconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigureOpenCodeAndRestore(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cfgDir := filepath.Join(tempHome, ".config", "opencode")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	initialJSON := `{
  "model": "gateway/pinball",
  "provider": {
    "gateway": {
      "options": {
        "baseURL": "https://opencode.agentgateway.grizzlygeek.net/v1",
        "apiKey": "agentgateway"
      }
    }
  }
}`
	path := filepath.Join(cfgDir, "opencode.json")
	if err := os.WriteFile(path, []byte(initialJSON), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	targetURL := "http://localhost:3000/v1"
	if err := ConfigureOpenCode(targetURL, false); err != nil {
		t.Fatalf("ConfigureOpenCode failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	var root map[string]interface{}
	json.Unmarshal(data, &root)
	provider := root["provider"].(map[string]interface{})
	gw := provider["gateway"].(map[string]interface{})
	opts := gw["options"].(map[string]interface{})

	if opts["baseURL"] != targetURL {
		t.Errorf("expected baseURL %s, got %v", targetURL, opts["baseURL"])
	}

	backupPath := path + ".convergence.bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("expected backup file to exist")
	}

	if err := ConfigureOpenCode("", true); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	restoredData, _ := os.ReadFile(path)
	if !strings.Contains(string(restoredData), "https://opencode.agentgateway.grizzlygeek.net/v1") {
		t.Errorf("restored content did not match original URL")
	}

	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Errorf("expected backup file to be removed after restore")
	}
}

func TestConfigureClaudeCodeAndRestore(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	targetURL := "http://localhost:3000/v1"
	if err := ConfigureClaudeCode(targetURL, false); err != nil {
		t.Fatalf("ConfigureClaudeCode failed: %v", err)
	}

	path, _ := GetClaudeCodeSettingsPath()
	data, _ := os.ReadFile(path)
	var root map[string]interface{}
	json.Unmarshal(data, &root)
	env := root["env"].(map[string]interface{})

	if env["ANTHROPIC_BASE_URL"] != targetURL {
		t.Errorf("expected ANTHROPIC_BASE_URL %s, got %v", targetURL, env["ANTHROPIC_BASE_URL"])
	}

	if err := ConfigureClaudeCode("", true); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
}

func TestConfigureAiderAndRestore(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	targetURL := "http://localhost:3000/v1"
	if err := ConfigureAider(targetURL, false); err != nil {
		t.Fatalf("ConfigureAider failed: %v", err)
	}

	path, _ := GetAiderConfigPath()
	data, _ := os.ReadFile(path)
	var root map[string]interface{}
	yaml.Unmarshal(data, &root)

	if root["openai-api-base"] != targetURL {
		t.Errorf("expected openai-api-base %s, got %v", targetURL, root["openai-api-base"])
	}

	if err := ConfigureAider("", true); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
}

func TestConfigureContinueAndRestore(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	targetURL := "http://localhost:3000/v1"
	if err := ConfigureContinue(targetURL, false); err != nil {
		t.Fatalf("ConfigureContinue failed: %v", err)
	}

	path, _ := GetContinueConfigPath()
	data, _ := os.ReadFile(path)
	var root map[string]interface{}
	json.Unmarshal(data, &root)
	models := root["models"].([]interface{})

	if len(models) == 0 {
		t.Fatalf("expected models array to contain Convergence")
	}

	m := models[0].(map[string]interface{})
	if m["apiBase"] != targetURL {
		t.Errorf("expected apiBase %s, got %v", targetURL, m["apiBase"])
	}

	if err := ConfigureContinue("", true); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
}
