package sandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

func TestGetDriver(t *testing.T) {
	cfg := config.DefaultConfig()

	dNone, err := GetDriver(config.SandboxNone, cfg)
	if err != nil || dNone.Type() != config.SandboxNone {
		t.Fatalf("expected SandboxNone driver, got %v, err %v", dNone, err)
	}

	dOS, err := GetDriver(config.SandboxOpenShell, cfg)
	if err != nil || dOS.Type() != config.SandboxOpenShell {
		t.Fatalf("expected SandboxOpenShell driver, got %v, err %v", dOS, err)
	}
	if dOS.InferenceURL() != "https://inference.local" {
		t.Errorf("unexpected InferenceURL: %s", dOS.InferenceURL())
	}

	dSbx, err := GetDriver(config.SandboxDockerSbx, cfg)
	if err != nil || dSbx.Type() != config.SandboxDockerSbx {
		t.Fatalf("expected SandboxDockerSbx driver, got %v, err %v", dSbx, err)
	}

	dK8s, err := GetDriver(config.SandboxK8sAgent, cfg)
	if err != nil || dK8s.Type() != config.SandboxK8sAgent {
		t.Fatalf("expected SandboxK8sAgent driver, got %v, err %v", dK8s, err)
	}

	dNono, err := GetDriver(config.SandboxNono, cfg)
	if err != nil || dNono.Type() != config.SandboxNono {
		t.Fatalf("expected SandboxNono driver, got %v, err %v", dNono, err)
	}
	if dNono.InferenceURL() != "http://127.0.0.1:3000/v1" {
		t.Errorf("unexpected nono InferenceURL: %s", dNono.InferenceURL())
	}
	cmd, env, err := dNono.RunCommand("opencode", []string{"--foo"})
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	if len(cmd) < 4 || cmd[0] != "nono" || cmd[1] != "run" {
		t.Errorf("unexpected RunCommand result: %v", cmd)
	}
	if len(env) == 0 {
		t.Errorf("expected env vars for nono, got none")
	}

	_, err = GetDriver(config.SandboxType("unknown"), cfg)
	if err == nil {
		t.Fatal("expected error for unknown driver, got nil")
	}
}

func TestOpenShellDriverStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := &openShellDriver{cfg: &config.Config{BindAddress: "127.0.0.1"}}
	res, err := d.Status(context.Background())
	if err != nil {
		t.Fatalf("Status returned unexpected error: %v", err)
	}
	if res.Type != config.SandboxOpenShell {
		t.Errorf("expected SandboxOpenShell type, got %s", res.Type)
	}
}
