package cli

import (
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

const testDSN = "postgres://u:hunter2@db:5432/ag" //nolint:gosec // fake DSN fixture

func TestOptimize_when_noDSN(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	err := runCLI(t, "optimize")
	if err == nil || !strings.Contains(err.Error(), "--db-dsn") || !strings.Contains(err.Error(), "EXPERIMENTAL") {
		t.Fatalf("got %v", err)
	}
	if e.calls(t) != "" {
		t.Error("engine must not run")
	}
}

func TestOptimize_when_notInitialized(t *testing.T) {
	newFakeEnv(t, "docker")
	if err := runCLI(t, "optimize", "--db-dsn", testDSN); err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("got %v", err)
	}
}

func TestOptimize_args(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		tail  string
	}{
		{"default", nil, "pinball-optimize run"},
		{"dry run", []string{"--dry-run"}, "pinball-optimize --dry-run run"},
		{"check revert wins", []string{"--check-revert", "--dry-run"}, "pinball-optimize --check-revert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newFakeEnv(t, "docker")
			e.saveCfg(t, func(c *config.Config) { c.GatewayPort = 3456 })
			if err := runCLI(t, append([]string{"optimize", "--db-dsn", testDSN}, tc.flags...)...); err != nil {
				t.Fatal(err)
			}
			calls := strings.TrimSpace(e.calls(t))
			mustContain(t, calls, "run --rm", "PINBALL_OPT_GATEWAY_URL=http://host.docker.internal:3456", tc.tail)
			if strings.Contains(calls, "hunter2") || strings.Contains(calls, "postgres://") {
				t.Errorf("DSN leaked into argv: %s", calls)
			}
			mustContain(t, calls, "-e PINBALL_OPT_DB_DSN ")
			mustContain(t, e.envLog(t), "DSN="+testDSN)
			if tc.name == "check revert wins" && strings.Contains(calls, "dry-run") {
				t.Error("dry-run should be ignored with check-revert")
			}
		})
	}
}

func TestOptimize_addHostForEachEngine(t *testing.T) {
	for _, eng := range []string{"docker", "podman"} {
		t.Run(eng, func(t *testing.T) {
			e := newFakeEnv(t, eng)
			e.saveCfg(t, func(c *config.Config) { c.Engine = eng })
			if err := runCLI(t, "optimize", "--db-dsn", testDSN); err != nil {
				t.Fatal(err)
			}
			mustContain(t, e.calls(t), "--add-host=host.docker.internal:host-gateway")
		})
	}
}

func TestOptimize_when_runFails(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	t.Setenv("FAKE_FAIL", "pinball-optimize")
	if err := runCLI(t, "optimize", "--db-dsn", testDSN); err == nil || !strings.Contains(err.Error(), "optimizer run failed") {
		t.Fatalf("got %v", err)
	}
}
