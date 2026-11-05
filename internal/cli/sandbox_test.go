package cli

import (
	"bytes"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestSandboxListCmd(t *testing.T) {
	buf := new(bytes.Buffer)
	sandboxListCmd.SetOut(buf)
	sandboxListCmd.Run(sandboxListCmd, []string{})
}

func TestResolveConfigSandbox(t *testing.T) {
	cfg := config.DefaultConfig()
	p := &prompter{yes: true}
	flags := initFlags{sandbox: "openshell"}
	if err := resolveConfig(cfg, flags, p); err != nil {
		t.Fatalf("resolveConfig failed: %v", err)
	}
	if cfg.Sandbox != config.SandboxOpenShell {
		t.Errorf("expected SandboxOpenShell, got %s", cfg.Sandbox)
	}

	flagsNono := initFlags{sandbox: "nono"}
	if err := resolveConfig(cfg, flagsNono, p); err != nil {
		t.Fatalf("resolveConfig failed: %v", err)
	}
	if cfg.Sandbox != config.SandboxNono {
		t.Errorf("expected SandboxNono, got %s", cfg.Sandbox)
	}

	flagsBad := initFlags{sandbox: "invalid-runtime"}
	if err := resolveConfig(cfg, flagsBad, p); err == nil {
		t.Errorf("expected error for invalid sandbox runtime, got nil")
	}
}
