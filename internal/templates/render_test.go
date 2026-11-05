package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestRenderAssetsCopilot(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderCopilot
	cfg.CopilotToken = "test-token"

	if err := RenderAssets(cfg, tempDir); err != nil {
		t.Fatalf("RenderAssets failed: %v", err)
	}

	composeData, err := os.ReadFile(filepath.Join(tempDir, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose: %v", err)
	}
	if !strings.Contains(string(composeData), "copilot-api") {
		t.Errorf("expected docker-compose to contain copilot-api")
	}

	gwData, err := os.ReadFile(filepath.Join(tempDir, "agentgateway.yaml"))
	if err != nil {
		t.Fatalf("read agentgateway: %v", err)
	}
	if !strings.Contains(string(gwData), "stand-in:9101") {
		t.Errorf("expected agentgateway.yaml to contain stand-in endpoint")
	}
	if !strings.Contains(string(gwData), "pinball:9201") {
		t.Errorf("expected agentgateway.yaml to contain pinball check")
	}

	pinData, err := os.ReadFile(filepath.Join(tempDir, "pinball.yaml"))
	if err != nil {
		t.Fatalf("read pinball: %v", err)
	}
	if !strings.Contains(string(pinData), "claude-haiku-5.5") {
		t.Errorf("expected pinball.yaml to contain models")
	}
}

func TestRenderAssetsDirect(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider = config.ProviderDirect
	cfg.OpenAIKey = "sk-test"
	cfg.AnthropicKey = "sk-ant-test"

	if err := RenderAssets(cfg, tempDir); err != nil {
		t.Fatalf("RenderAssets failed: %v", err)
	}

	composeData, err := os.ReadFile(filepath.Join(tempDir, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose: %v", err)
	}
	if strings.Contains(string(composeData), "copilot-api") {
		t.Errorf("expected docker-compose to NOT contain copilot-api for direct provider")
	}
}
