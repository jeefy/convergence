package engine

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const script = `#!/bin/sh
echo "$@" >> "$FAKE_LOG"
[ -n "$FAKE_PROVIDER_LOG" ] && echo "$PODMAN_COMPOSE_PROVIDER" >> "$FAKE_PROVIDER_LOG"
[ -n "$FAKE_FAIL" ] && case "$*" in *"$FAKE_FAIL"*) echo boom >&2; exit 3;; esac
case "$*" in ps\ *|*" ps "*)
  [ -n "$FAKE_WARNING" ] && echo "$FAKE_WARNING" >&2
  for s in $FAKE_RUNNING; do echo "$s"; done
  exit 0;;
esac
exit 0
`

func fakeBins(t *testing.T, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(bin, n), []byte(script), 0o700); err != nil { //nolint:gosec // test executable; G703 false positive
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_LOG", filepath.Join(t.TempDir(), "log"))
	t.Setenv("FAKE_FAIL", "")
	t.Setenv("FAKE_RUNNING", "gw pb")
	t.Setenv("FAKE_WARNING", "")
	return bin
}

func logOf(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(os.Getenv("FAKE_LOG")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDetectEngine_order(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed []string
		preferred string
		want      string
	}{
		{"preferred wins", []string{"docker", "podman"}, "podman", "podman"},
		{"docker before podman", []string{"docker", "podman"}, "", "docker"},
		{"podman fallback", []string{"podman"}, "", "podman"},
		{"preferred missing falls back", []string{"docker"}, "podman", "docker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeBins(t, tc.installed...)
			e, err := DetectEngine(tc.preferred)
			if err != nil || e.Name() != tc.want {
				t.Fatalf("got %v, %v want %s", e, err, tc.want)
			}
		})
	}
}

func TestCompose_commandSelection(t *testing.T) {
	fakeBins(t, "podman")
	e, err := DetectEngine("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ctx := t.Context()
	for name, fn := range map[string]func(context.Context, string) error{
		"up -d":                    e.Start,
		"down":                     e.Stop,
		"restart":                  e.Restart,
		"down -v --remove-orphans": e.Down,
		"pull":                     e.Pull,
	} {
		if err := fn(ctx, dir); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := e.Logs(ctx, dir, true, "pinball"); err != nil {
		t.Fatal(err)
	}
	got := logOf(t)
	f := filepath.Join(dir, "docker-compose.yaml")
	for _, want := range []string{
		"compose -f " + f + " up -d", "compose -f " + f + " restart", "down -v --remove-orphans",
		"pull", "logs -f pinball",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestComposeProviderSelection(t *testing.T) {
	for _, provider := range []string{"", "auto", "podman-compose", "docker-compose"} {
		t.Run(provider, func(t *testing.T) {
			bin := fakeBins(t, "podman", "podman-compose", "docker-compose")
			t.Setenv("PODMAN_COMPOSE_PROVIDER", "inherited-provider")
			log := filepath.Join(t.TempDir(), "provider.log")
			t.Setenv("FAKE_PROVIDER_LOG", log)
			e, err := DetectEngine("podman")
			if err != nil {
				t.Fatal(err)
			}
			if err := e.SetComposeProvider(provider); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			for _, operation := range []func(context.Context, string) error{
				e.Start, e.Stop, e.Restart, e.Down, e.Pull,
			} {
				if err := operation(t.Context(), dir); err != nil {
					t.Fatal(err)
				}
			}
			if err := e.Logs(t.Context(), dir, true, "pinball"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			want := "inherited-provider"
			if provider != "" && provider != "auto" {
				want = filepath.Join(bin, provider)
			}
			if got := string(data); got != strings.Repeat(want+"\n", 6) {
				t.Fatalf("provider selection: %q, want six uses of %q", got, want)
			}
			if _, err := e.RunningServices(t.Context(), dir); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(data), "inherited-provider\n") {
				t.Fatal("native podman ps must not receive the Compose override")
			}
		})
	}
}

func TestComposeProviderRejectsUnavailableSelection(t *testing.T) {
	for _, tc := range []struct {
		engine, provider, wantErr string
	}{
		{"podman", "podman-compose", "not available on PATH"},
		{"docker", "podman-compose", "requires engine podman"},
		{"podman", "invalid", "invalid compose_provider"},
	} {
		t.Run(tc.engine+"/"+tc.provider, func(t *testing.T) {
			fakeBins(t, tc.engine)
			e, err := DetectEngine(tc.engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := e.SetComposeProvider(tc.provider); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCompose_errorWrapping(t *testing.T) {
	fakeBins(t, "docker")
	t.Setenv("FAKE_FAIL", "compose")
	e, err := DetectEngine("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, err := range map[string]error{
		"start":   e.Start(t.Context(), dir),
		"pull":    e.Pull(t.Context(), dir),
		"logs":    e.Logs(t.Context(), dir, false),
		"restart": e.Restart(t.Context(), dir),
	} {
		if err == nil || !strings.Contains(err.Error(), "docker compose") || !strings.Contains(err.Error(), "failed") {
			t.Errorf("%s: %v", name, err)
		}
		if !strings.Contains(err.Error(), "exit status 3") {
			t.Errorf("%s: cause not wrapped: %v", name, err)
		}
	}
	if err := e.Start(t.Context(), dir); !strings.Contains(err.Error(), "boom") {
		t.Errorf("output not included: %v", err)
	}
	t.Setenv("FAKE_FAIL", "")
	if _, err := e.RunningServices(t.Context(), dir); err != nil {
		t.Fatal(err)
	}
}

func TestRunningServices(t *testing.T) {
	for _, name := range []string{"docker", "podman"} {
		t.Run(name, func(t *testing.T) {
			fakeBins(t, name)
			e, err := DetectEngine("")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			t.Setenv("FAKE_WARNING", "engine warning")
			got, err := e.RunningServices(t.Context(), dir)
			if err != nil || len(got) != 2 || got[0] != "gw" || got[1] != "pb" {
				t.Fatalf("got %v %v", got, err)
			}
			want := "compose -f " + filepath.Join(dir, "docker-compose.yaml") + " ps --services --filter status=running"
			if name == "podman" {
				want = "ps --filter label=com.docker.compose.project.config_files=" +
					filepath.Join(dir, "docker-compose.yaml") +
					` --filter status=running --format {{.Label "com.docker.compose.service"}}`
			}
			if got := strings.TrimSpace(logOf(t)); got != want {
				t.Fatalf("command = %q, want %q", got, want)
			}

			t.Setenv("FAKE_RUNNING", "")
			got, err = e.RunningServices(t.Context(), dir)
			if err != nil || len(got) != 0 {
				t.Fatalf("idle stack: got %v %v", got, err)
			}

			t.Setenv("FAKE_FAIL", "ps")
			_, err = e.RunningServices(t.Context(), dir)
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 || !strings.Contains(err.Error(), "boom") {
				t.Fatalf("expected wrapped exit error and stderr, got %v", err)
			}
		})
	}
	bad := &ComposeEngine{binary: filepath.Join(t.TempDir(), "missing")}
	if _, err := bad.RunningServices(t.Context(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "ps failed") {
		t.Fatalf("got %v", err)
	}
}

func TestPortConflictError_unwrapsAndNamesPort(t *testing.T) {
	inner := errors.New("bind: address already in use")
	err := &PortConflictError{Service: "pinball", Addr: "127.0.0.1:9201", Err: inner}
	if !errors.Is(err, inner) || !strings.Contains(err.Error(), "127.0.0.1:9201") || !strings.Contains(err.Error(), "pinball") {
		t.Errorf("bad: %v", err)
	}
}

func TestCheckHealth(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	host, ps, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(ps)
	e := &ComposeEngine{binary: "docker"}

	if err := e.CheckHealth(t.Context(), host, port); err != nil {
		t.Fatal(err)
	}
	status = http.StatusServiceUnavailable
	if err := e.CheckHealth(t.Context(), host, port); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("got %v", err)
	}
	if err := e.CheckHealth(t.Context(), "127.0.0.1", 1); err == nil || !strings.Contains(err.Error(), "connection failed") {
		t.Fatalf("got %v", err)
	}
}

func TestWaitHealthy_timesOutWithLastError(t *testing.T) {
	e := &ComposeEngine{binary: "docker"}
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	err := e.WaitHealthy(ctx, "127.0.0.1", 1)
	if err == nil || !strings.Contains(err.Error(), "not healthy") || !strings.Contains(err.Error(), "connection failed") {
		t.Fatalf("got %v", err)
	}
}

func TestWaitHealthy_succeedsAfterRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	host, ps, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(ps)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := (&ComposeEngine{binary: "docker"}).WaitHealthy(ctx, host, port); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPortsFree_stopsAtFirstConflict(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	taken := ln.Addr().(*net.TCPAddr).Port
	err = CheckPortsFree("127.0.0.1", namedPorts(taken))
	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(taken)) {
		t.Fatalf("got %v", err)
	}
}
