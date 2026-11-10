package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestResolveConfig_allFlags(t *testing.T) {
	cfg := config.DefaultConfig()
	f := initFlags{
		yes: true, provider: "ollama", ollamaURL: "http://o:1", port: 4001, failOpen: true,
		requestLogDB: "postgres://x", otel: config.OTel{Endpoint: "c:4317", Protocol: "grpc", Sample: 0.5},
		ollamaLight: "l", ollamaStd: "s", ollamaHeavy: "h",
		imageAGW: "a", imageStandIn: "b", imagePinball: "c", imageCopilotAPI: "d", imageOpenShell: "e",
	}
	if err := resolveConfig(cfg, f, newPrompter("", true)); err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaURL != "http://o:1" || !cfg.FailOpen || cfg.OllamaModelHeavy != "h" || cfg.ImageOpenShellGateway != "e" || cfg.OTel.Sample != 0.5 {
		t.Errorf("flags not applied: %+v", *cfg)
	}
}

func TestResolveConfig_errors(t *testing.T) {
	for name, f := range map[string]initFlags{
		"port":     {yes: true, provider: "ollama", port: 70000},
		"otel":     {yes: true, provider: "ollama", otel: config.OTel{Endpoint: "c", Protocol: "bogus"}},
		"provider": {yes: true, provider: "nope"},
	} {
		if err := resolveConfig(config.DefaultConfig(), f, newPrompter("", true)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestResolveConfig_ollamaPrompt(t *testing.T) {
	cfg := config.DefaultConfig()
	if err := resolveConfig(cfg, initFlags{provider: "ollama"}, newPrompter("http://p:2\n", false)); err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaURL != "http://p:2" {
		t.Errorf("url %q", cfg.OllamaURL)
	}
}

func TestDetectInitEngine(t *testing.T) {
	newFakeEnv(t, "docker")
	if _, err := detectInitEngine("bogus"); err == nil {
		t.Error("invalid engine accepted")
	}
	if _, err := detectInitEngine("podman"); err == nil {
		t.Error("podman absent should error")
	}
	if e, err := detectInitEngine("docker"); err != nil || e.Name() != "docker" {
		t.Errorf("docker: %v", err)
	}
	if _, err := detectInitEngine(""); err != nil {
		t.Error(err)
	}
}

func TestWriteStarterModels_noClobber(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	if err := writeStarterModels(dir); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "models.yaml")
	if err := os.WriteFile(p, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeStarterModels(dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "keep" {
		t.Error("clobbered")
	}
	if err := writeStarterModels(p); err == nil {
		t.Error("expected mkdir error on file path")
	}
}

func TestRunDoctor_withConfig(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := healthyGateway(t)
	e.saveCfg(t, func(c *config.Config) { c.Provider = config.ProviderDirect; c.GatewayPort = port })
	res := runDoctor(context.Background())
	if len(res) < 2 {
		t.Fatalf("expected full checks, got %d", len(res))
	}
	svc := newServiceChecker(config.DefaultConfig(), "")
	_ = svc.Check(context.Background())
}

func writeFakeKubectl(t *testing.T, exit string) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "k.log")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\ncat >/dev/null 2>&1 < /dev/null\nexit " + exit + "\n"
	if err := os.WriteFile(filepath.Join(bin, "kubectl"), []byte(script), 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())
	return log
}

func TestCluster_commandsWithFakeKubectl(t *testing.T) {
	log := writeFakeKubectl(t, "0")
	for _, args := range [][]string{
		{"cluster", "install", "--provider", "direct", "--openai-key", "sk-test", "--dry-run"},
		{"cluster", "install", "--provider", "direct", "--openai-key", "sk-test", "--no-backup"},
		{"cluster", "uninstall", "--yes"},
		{"cluster", "status"},
		{"cluster", "logs", "pinball", "--tail", "5", "-f"},
		{"cluster", "logs"},
	} {
		resetFlags(rootCmd)
		if err := runCLI(t, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	b, _ := os.ReadFile(log)
	mustContain(t, string(b), "apply -f -", "delete namespace", "get pods", "app=pinball", "--tail=5")
}

func TestCluster_installCopilotNeedsToken(t *testing.T) {
	writeFakeKubectl(t, "1") // no existing Secret to reuse
	resetFlags(rootCmd)
	err := runCLI(t, "cluster", "install", "--provider", "copilot", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "--copilot-token") {
		t.Fatalf("expected missing-token error, got %v", err)
	}
}

func TestCluster_kubectlFailures(t *testing.T) {
	writeFakeKubectl(t, "1")
	for _, args := range [][]string{
		{"cluster", "install", "--provider", "direct", "--openai-key", "sk-test", "--no-backup"},
		{"cluster", "uninstall", "--yes"},
		{"cluster", "status"},
		{"cluster", "logs"},
	} {
		resetFlags(rootCmd)
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}
