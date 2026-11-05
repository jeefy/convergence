package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeefy/convergence/internal/models"
)

func TestModelsInit_doesNotClobber(t *testing.T) {
	e := newFakeEnv(t)
	if err := os.MkdirAll(e.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(e.dir, models.FileName)
	if err := os.WriteFile(path, []byte("mine: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	if err := runCLI(t, "models", "init"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil || string(b) != "mine: true\n" {
		t.Fatalf("clobbered: %q %v", b, err)
	}
	mustContain(t, out.String(), "already exists")
}

func TestModelsInit_createsStarter(t *testing.T) {
	e := newFakeEnv(t)
	if err := runCLI(t, "models", "init"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.dir, models.FileName))
	if err != nil || string(b) != models.StarterFile() {
		t.Fatalf("starter not written: %v", err)
	}
}

func TestModels_listsTiersForProvider(t *testing.T) {
	e := newFakeEnv(t)
	e.saveCfg(t, nil)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := runCLI(t, "models"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, out.String(), "Provider: direct", "TIER", "UPSTREAM")
}

func TestModels_when_badOverrideFile(t *testing.T) {
	e := newFakeEnv(t)
	e.saveCfg(t, nil)
	if err := os.WriteFile(filepath.Join(e.dir, models.FileName), []byte("direct: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "models"); err == nil {
		t.Fatal("want error for corrupt models.yaml")
	}
}

func TestModels_when_corruptConfig(t *testing.T) {
	e := newFakeEnv(t)
	if err := os.MkdirAll(e.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "config.yaml"), []byte("::: bad\n\t-"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "models"); err == nil {
		t.Fatal("want error")
	}
}
