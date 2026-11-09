package cli

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jeefy/convergence/internal/config"
)

const fakeEngineScript = `#!/bin/sh
echo "$@" >> "$FAKE_LOG"
echo "DSN=$PINBALL_OPT_DB_DSN" >> "$FAKE_LOG.env"
echo "COMPOSE_PROVIDER=$PODMAN_COMPOSE_PROVIDER" >> "$FAKE_LOG.env"
case "$*" in
  ps\ *|*" ps "*) for s in $FAKE_RUNNING; do echo "$s"; done; exit 0;;
  info\ *RemoteSocket.Path*) echo "$FAKE_SOCKET"; exit 0;;
esac
if [ -n "$FAKE_FAIL" ]; then
  case "$*" in *"$FAKE_FAIL"*) echo boom >&2; exit 1;; esac
fi
exit 0
`

// fakeEnv is a sandboxed HOME plus a fake engine binary on PATH.
type fakeEnv struct {
	home string
	bin  string
	dir  string // ~/.convergence
	log  string
}

// newFakeEnv installs fake binaries named engines into a PATH-only temp dir.
func newFakeEnv(t *testing.T, engines ...string) *fakeEnv {
	t.Helper()
	home := t.TempDir()
	bin := t.TempDir()
	for _, name := range engines {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(fakeEngineScript), 0o700); err != nil { //nolint:gosec // test executable
			t.Fatal(err)
		}
	}
	logf := filepath.Join(t.TempDir(), "calls.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_LOG", logf)
	t.Setenv("FAKE_FAIL", "")
	t.Setenv("FAKE_RUNNING", "agentgateway")
	return &fakeEnv{home: home, bin: bin, dir: filepath.Join(home, ".convergence"), log: logf}
}

func (e *fakeEnv) calls(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (e *fakeEnv) envLog(t *testing.T) string { //nolint:unused // test helper kept for env-log assertions being added concurrently
	t.Helper()
	b, err := os.ReadFile(e.log + ".env")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// saveCfg writes a config for ollama-free provider "direct" by default.
func (e *fakeEnv) saveCfg(t *testing.T, mutate func(*config.Config)) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.OpenAIKey = "sk-test"
	if mutate != nil {
		mutate(cfg)
	}
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

// healthyGateway serves /v1/models and points cfg at it.
func healthyGateway(t *testing.T) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return portOf(t, srv.Listener.Addr())
}

func portOf(t *testing.T, a net.Addr) int {
	t.Helper()
	_, p, err := net.SplitHostPort(a.String())
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := portOf(t, ln.Addr())
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func resetFlags(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil) // Set("[]") would append a literal "[]" element
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

// runCLI executes the root command with args and resets flag state after.
func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() { resetFlags(rootCmd); rootCmd.SetArgs(nil) })
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// withStdin replaces os.Stdin with the given content for the test.
func withStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = r.Close() })
}

func mustContain(t *testing.T, got string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
}
