package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigClaudeCode_configureDryRunRestore(t *testing.T) {
	e := newFakeEnv(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	path := filepath.Join(e.home, ".claude", "settings.json")

	if err := runCLI(t, "config", "claude-code", "--dry-run", "--gateway-url", "http://gw:3000"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, out.String(), "[dry-run]")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("dry-run wrote a file")
	}

	out.Reset()
	resetFlags(rootCmd)
	if err := runCLI(t, "config", "claude-code", "--gateway-url", "http://gw:3000"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, out.String(), "configured")
	b, err := os.ReadFile(path) //nolint:gosec // test path under temp HOME
	if err != nil || !strings.Contains(string(b), "gw:3000") {
		t.Fatalf("not written: %q %v", b, err)
	}

	out.Reset()
	resetFlags(rootCmd)
	if err := runCLI(t, "config", "claude-code", "--gateway-url", "http://gw:3000"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, out.String(), "already up to date")

	out.Reset()
	resetFlags(rootCmd)
	if err := runCLI(t, "config", "claude-code", "restore", "--gateway-url", "http://gw:3000"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, out.String(), "restored")
}

func TestConfigRejectsUnknownArgAndEnvPrints(t *testing.T) {
	newFakeEnv(t)
	if err := runCLI(t, "config", "aider", "bogus"); err == nil || !strings.Contains(err.Error(), "unknown argument") {
		t.Errorf("want unknown argument error, got %v", err)
	}
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := runCLI(t, "config", "env"); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Error("env printed nothing")
	}
}
