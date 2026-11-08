package models

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func writeModels(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func unreadableModelsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// A directory named models.yaml makes ReadFile fail with a non-ENOENT error.
	if err := os.Mkdir(filepath.Join(dir, FileName), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadOverrides_missingFileIsEmpty(t *testing.T) {
	o, err := LoadOverrides(t.TempDir())
	if err != nil || len(o) != 0 {
		t.Fatalf("o=%v err=%v", o, err)
	}
}

func TestLoadOverrides_readsFile(t *testing.T) {
	o, err := LoadOverrides(writeModels(t, "copilot:\n  claude-haiku-5.5: gpt-x\n"))
	if err != nil || o["copilot"]["claude-haiku-5.5"] != "gpt-x" {
		t.Fatalf("o=%v err=%v", o, err)
	}
}

func TestLoadOverrides_readErrorAndParseError(t *testing.T) {
	if _, err := LoadOverrides(unreadableModelsDir(t)); err == nil {
		t.Error("want read error")
	}
	if _, err := LoadOverrides(writeModels(t, "copilot: [1]\n")); err == nil {
		t.Error("want parse error")
	}
}

func TestLoadPinball_missingReadAndParse(t *testing.T) {
	if pb, err := loadPinball(t.TempDir()); pb != nil || err != nil {
		t.Errorf("missing: pb=%v err=%v", pb, err)
	}
	if _, err := loadPinball(unreadableModelsDir(t)); err == nil {
		t.Error("want read error")
	}
	if _, err := loadPinball(writeModels(t, "pinball: [1]\n")); err == nil {
		t.Error("want parse error")
	}
}

func TestParseOverrides_rejectsBadYAMLAndShape(t *testing.T) {
	if _, err := ParseOverrides([]byte(": :\n\t-")); err == nil {
		t.Error("want yaml error")
	}
	if _, err := ParseOverrides([]byte("copilot: scalar\n")); err == nil {
		t.Error("want decode error")
	}
}

func TestForConfig_errorPaths(t *testing.T) {
	cfg := config.DefaultConfig()
	if _, err := ForConfig(cfg, unreadableModelsDir(t)); err == nil {
		t.Error("want read error")
	}
	if _, err := ForConfig(cfg, writeModels(t, "pinball: [1]\n")); err == nil {
		t.Error("want pinball error")
	}
	if _, err := ForConfig(cfg, writeModels(t, "passthrough:\n  auto:\n    provider: openai\n    model: m\n")); err == nil {
		t.Error("want passthrough error")
	}
	bad := config.DefaultConfig()
	bad.Provider = "bogus"
	if _, err := ForConfig(bad, t.TempDir()); err == nil {
		t.Error("want resolve error")
	}
}

func TestSplitSpec_branches(t *testing.T) {
	for spec, want := range map[string]string{
		"ollama:m": UpstreamOllama, "copilot:m": UpstreamCopilot, "openai:m": UpstreamOpenAI,
	} {
		if f, _, err := splitSpec(config.ProviderDirect, spec); err != nil || f != want {
			t.Errorf("%s: %s %v", spec, f, err)
		}
	}
	if _, _, err := splitSpec(config.ProviderDirect, "gemini:m"); err == nil {
		t.Error("want unsupported family")
	}
	if _, _, err := splitSpec("bogus", "m"); err == nil {
		t.Error("want unknown provider")
	}
}
