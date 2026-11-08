package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/models"
)

const metricsBody = "# TYPE agentgateway_requests_total counter\nagentgateway_requests_total{status=\"200\"} 7\n"

func metricsServer(t *testing.T, status int) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(metricsBody))
	}))
	t.Cleanup(srv.Close)
	return portOf(t, srv.Listener.Addr())
}

func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
}

func corruptConfig(t *testing.T, e *fakeEnv) {
	t.Helper()
	if err := os.MkdirAll(e.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "config.yaml"), []byte("provider: [unterminated\n\t:"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStats_scrapePrintsAndJSON(t *testing.T) {
	newFakeEnv(t, "docker")
	gw, si, pb := metricsServer(t, 200), metricsServer(t, 200), metricsServer(t, 500)
	ports := []string{"--gateway-metrics-port", strconv.Itoa(gw), "--standin-metrics-port", strconv.Itoa(si), "--pinball-metrics-port", strconv.Itoa(pb)}

	if err := runCLI(t, append([]string{"stats"}, ports...)...); err != nil {
		t.Fatalf("partial availability must not fail: %v", err)
	}
	resetFlags(rootCmd)
	if err := runCLI(t, append([]string{"stats", "--json"}, ports...)...); err != nil {
		t.Fatal(err)
	}
}

func TestStats_allDownErrors(t *testing.T) {
	newFakeEnv(t, "docker")
	p := strconv.Itoa(freePort(t))
	err := runCLI(t, "stats", "--gateway-metrics-port", p, "--standin-metrics-port", p, "--pinball-metrics-port", p)
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("got %v", err)
	}
}

func TestStats_watchStopsOnContextCancel(t *testing.T) {
	newFakeEnv(t, "docker")
	gw := metricsServer(t, 200)
	// Cobra keeps a subcommand's context from earlier runs, so set it directly.
	t.Cleanup(func() { statsCmd.SetContext(context.Background()) })
	for _, js := range []bool{true, false} {
		resetFlags(rootCmd)
		args := []string{"stats", "--watch", "--interval", "1", "--gateway-metrics-port", strconv.Itoa(gw)}
		if js {
			args = append(args, "--json")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		rootCmd.SetArgs(args)
		statsCmd.SetContext(ctx)
		err := rootCmd.Execute()
		cancel()
		if err != nil {
			t.Fatalf("json=%v: %v", js, err)
		}
	}
	t.Cleanup(func() { resetFlags(rootCmd); rootCmd.SetArgs(nil) })
}

func TestStatus_downReturnsErrorAndTextOutput(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = freePort(t) })
	t.Setenv("FAKE_RUNNING", "")
	err := runCLI(t, "status")
	if err == nil || !strings.Contains(err.Error(), "required services are down") {
		t.Fatalf("got %v", err)
	}
	resetFlags(rootCmd)
	if err := runCLI(t, "status", "--json"); err == nil {
		t.Fatal("json mode must also fail when down")
	}
}

func TestStatus_healthyExitsZero(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := healthyGateway(t)
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
	t.Setenv("FAKE_RUNNING", "agentgateway stand-in pinball")
	// Other services may still probe unhealthy; only the command must run.
	_ = runCLI(t, "status")
}

func TestCorruptConfig_surfacesErrorEverywhere(t *testing.T) {
	for _, args := range [][]string{
		{"status"}, {"stop"}, {"down"}, {"logs"}, {"uninstall"}, {"upgrade"}, {"restart"}, {"start"},
		{"optimize", "--db-dsn", "postgres://u:p@h/db"}, {"models"}, {"sandbox", "status"}, {"sandbox", "run", "x"},
		{"init", "--yes", "--provider", "ollama"}, {"login", "copilot"},
		{"config", "claude-code"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			e := newFakeEnv(t, "docker")
			corruptConfig(t, e)
			if err := runCLI(t, args...); err == nil {
				t.Fatal("expected config load error")
			}
		})
	}
}

func TestNoHome_commandsFail(t *testing.T) {
	newFakeEnv(t, "docker")
	t.Setenv("HOME", "")
	for _, args := range [][]string{{"models", "init"}, {"doctor"}, {"models"}} {
		resetFlags(rootCmd)
		if err := runCLI(t, args...); err == nil {
			t.Errorf("%v: expected error without HOME", args)
		}
	}
}

func TestDoctor_runsAndReportsFailures(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = freePort(t) })
	t.Setenv("FAKE_RUNNING", "")
	err := runCLI(t, "doctor", "--timeout", "3s", "--json")
	if err == nil || !strings.Contains(err.Error(), "failing checks") {
		t.Fatalf("got %v", err)
	}
	resetFlags(rootCmd)
	if err := runCLI(t, "doctor", "--timeout", "3s"); err == nil {
		t.Fatal("text mode must fail too")
	}
	resetFlags(rootCmd)
	e2 := newFakeEnv(t, "docker")
	_ = e2
	if err := runCLI(t, "doctor", "--timeout", "3s"); err == nil {
		t.Fatal("uninitialised config must fail doctor")
	}
}

func TestSandboxStatus_perRuntime(t *testing.T) {
	for _, tc := range []struct {
		rt     config.SandboxType
		nono   bool
		expect string
	}{
		{config.SandboxNone, false, "No sandboxing runtime"},
		{config.SandboxNono, true, "nono CLI available"},
		{config.SandboxNono, false, "not found on PATH"},
		{config.SandboxK8sAgent, false, "agents.x-k8s.io"},
		{config.SandboxDockerSbx, false, "Inference URL"},
		{config.SandboxOpenShell, false, "unreachable"},
	} {
		t.Run(string(tc.rt), func(t *testing.T) {
			e := newFakeEnv(t, "docker")
			if tc.nono {
				bin := filepath.Dir(mustLook(t, "docker"))
				writeFakeBin(t, bin, "nono", "#!/bin/sh\nexit 0\n")
			}
			e.saveCfg(t, func(c *config.Config) { c.Sandbox = tc.rt })
			out := captureStdout(t, func() {
				if err := runCLI(t, "sandbox", "status"); err != nil {
					t.Fatal(err)
				}
			})
			mustContain(t, out, "Sandbox runtime: "+string(tc.rt), tc.expect)
		})
	}
}

func TestSandboxStatus_json(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.Sandbox = config.SandboxK8sAgent })
	out := captureStdout(t, func() {
		if err := runCLI(t, "sandbox", "status", "--json"); err != nil {
			t.Fatal(err)
		}
	})
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil || m["type"] != "k8s-agent" {
		t.Fatalf("bad json %v: %s", err, out)
	}
}

func TestSandboxRun(t *testing.T) {
	t.Run("none configured", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		if err := runCLI(t, "sandbox", "run", "agent"); err == nil || !strings.Contains(err.Error(), "no sandbox runtime") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("nono runs with mounts and env", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		bin := filepath.Dir(mustLook(t, "docker"))
		out := filepath.Join(t.TempDir(), "nono.out")
		writeFakeBin(t, bin, "nono", fmt.Sprintf("#!/bin/sh\necho \"$@ URL=$OPENAI_BASE_URL\" > %s\nexit 0\n", out))
		e.saveCfg(t, func(c *config.Config) { c.Sandbox = config.SandboxNono; c.GatewayPort = 3456 })
		if err := runCLI(t, "sandbox", "run", "-v", "/tmp/work", "agent", "arg1"); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(out)
		mustContain(t, string(b), "run --allow-cwd --allow-path /tmp/work -- agent arg1", "URL=http://127.0.0.1:3456/v1")
	})
	t.Run("nono exit status surfaces", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		writeFakeBin(t, filepath.Dir(mustLook(t, "docker")), "nono", "#!/bin/sh\nexit 3\n")
		e.saveCfg(t, func(c *config.Config) { c.Sandbox = config.SandboxNono })
		if err := runCLI(t, "sandbox", "run", "agent"); err == nil {
			t.Fatal("expected non-zero exit error")
		}
	})
	t.Run("binary missing", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, func(c *config.Config) { c.Sandbox = config.SandboxNono })
		err := runCLI(t, "sandbox", "run", "agent")
		if err == nil || !strings.Contains(err.Error(), "not found on PATH") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("k8s-agent cannot run locally", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, func(c *config.Config) { c.Sandbox = config.SandboxK8sAgent })
		if err := runCLI(t, "sandbox", "run", "agent"); err == nil || !strings.Contains(err.Error(), "Custom Resource") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestCheckOllama_branches(t *testing.T) {
	tags := func(names ...string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			var ms []map[string]string
			for _, n := range names {
				ms = append(ms, map[string]string{"name": n})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"models": ms})
		}))
	}
	t.Run("missing models warns then strict fails", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		srv := tags()
		defer srv.Close()
		port := healthyGateway(t)
		e.saveCfg(t, func(c *config.Config) {
			c.GatewayPort = port
			c.Provider = config.ProviderOllama
			c.OllamaURL = srv.URL
		})
		if err := runCLI(t, "start"); err != nil {
			t.Fatalf("non-strict must warn only: %v", err)
		}
		resetFlags(rootCmd)
		err := runCLI(t, "start", "--strict")
		if err == nil || !strings.Contains(err.Error(), "ollama pull") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("all models present", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		cfg := config.DefaultConfig()
		cfg.Provider = config.ProviderOllama
		res, err := models.ForConfig(cfg, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, m := range res.Models {
			names = append(names, m.UpstreamModel)
		}
		srv := tags(names...)
		defer srv.Close()
		port := healthyGateway(t)
		e.saveCfg(t, func(c *config.Config) {
			c.GatewayPort = port
			c.Provider = config.ProviderOllama
			c.OllamaURL = srv.URL
		})
		if err := runCLI(t, "start", "--strict"); err != nil {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("bad url", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, func(c *config.Config) {
			c.GatewayPort = healthyGateway(t)
			c.Provider = config.ProviderOllama
			c.OllamaURL = "http://bad host\x7f"
		})
		err := runCLI(t, "start", "--strict")
		if err == nil || !strings.Contains(err.Error(), "cannot check Ollama models") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("models.yaml invalid", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, func(c *config.Config) {
			c.GatewayPort = healthyGateway(t)
			c.Provider = config.ProviderOllama
		})
		if err := os.WriteFile(filepath.Join(e.dir, "models.yaml"), []byte("::: [bad"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := runCLI(t, "start"); err == nil {
			t.Fatal("expected models.yaml error")
		}
	})
}

func TestStart_healthTimeoutAndEngineFailure(t *testing.T) {
	old := healthTimeout
	healthTimeout = 300 * time.Millisecond
	t.Cleanup(func() { healthTimeout = old })
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = freePort(t) })
	err := runCLI(t, "start")
	if err == nil || !strings.Contains(err.Error(), "convergence logs agentgateway") {
		t.Fatalf("got %v", err)
	}
	t.Setenv("FAKE_FAIL", " ps ")
	resetFlags(rootCmd)
	if err := runCLI(t, "start"); err == nil {
		t.Fatal("ps failure must abort start")
	}
}

func TestInit_wizardPerProvider(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
		check       func(*config.Config) bool
	}{
		{"copilot", "1\nghp_abcdefghijklmnopqrstuvwxyz0123456789\n", func(c *config.Config) bool {
			return c.Provider == config.ProviderCopilot && c.CopilotToken == "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
		}},
		{"copilot skip token", "1\n\n", func(c *config.Config) bool { return c.Provider == config.ProviderCopilot && c.CopilotToken == "" }},
		{"direct", "2\nant\noai\n", func(c *config.Config) bool { return c.AnthropicKey == "ant" && c.OpenAIKey == "oai" }},
		{"ollama", "3\nhttp://o:11434\n", func(c *config.Config) bool {
			return c.Provider == config.ProviderOllama && c.OllamaURL == "http://o:11434"
		}},
		{"ollama default url", "3\n\n", func(c *config.Config) bool { return c.Provider == config.ProviderOllama }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newFakeEnv(t, "docker")
			withStdin(t, tc.stdin)
			if err := runCLI(t, "init"); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadConfig()
			if err != nil || !tc.check(cfg) {
				t.Fatalf("cfg mismatch (%v): %+v", err, cfg)
			}
			for _, f := range []string{"config.yaml", "docker-compose.yaml", "models.yaml"} {
				if _, err := os.Stat(filepath.Join(e.dir, f)); err != nil {
					t.Errorf("%s not written: %v", f, err)
				}
			}
		})
	}
}

func TestInit_invalidChoiceAndEngineErrors(t *testing.T) {
	newFakeEnv(t, "docker")
	withStdin(t, "9\n")
	if err := runCLI(t, "init"); err == nil || !strings.Contains(err.Error(), "invalid choice") {
		t.Fatalf("got %v", err)
	}
	for _, eng := range []string{"podman", "lxc"} {
		resetFlags(rootCmd)
		if err := runCLI(t, "init", "--yes", "--provider", "ollama", "--engine", eng); err == nil {
			t.Errorf("--engine %s should fail with only docker present", eng)
		}
	}
	resetFlags(rootCmd)
	if err := runCLI(t, "init", "--yes", "--provider", "ollama", "--engine", "docker"); err != nil {
		t.Fatal(err)
	}
}

func TestInit_noEngineAndFailOpenWarning(t *testing.T) {
	newFakeEnv(t)
	if err := runCLI(t, "init", "--yes", "--provider", "ollama"); err == nil {
		t.Fatal("no engine on PATH must fail")
	}
	resetFlags(rootCmd)
	newFakeEnv(t, "docker")
	out := captureStdout(t, func() {
		if err := runCLI(t, "init", "--yes", "--provider", "copilot", "--fail-open"); err != nil {
			t.Fatal(err)
		}
	})
	mustContain(t, out, "UNSCRUBBED", "convergence login copilot")
}

func TestSecretField_replaceFlow(t *testing.T) {
	p := newPrompter("n\n", false)
	if got, err := p.secretField("key", "new", "old"); err == nil || got != "old" {
		t.Fatalf("declined replace: %q %v", got, err)
	}
	p = newPrompter("y\n", false)
	if got, err := p.secretField("key", "new", "old"); err != nil || got != "new" {
		t.Fatalf("accepted replace: %q %v", got, err)
	}
	p = newPrompter("", false)
	if got, err := p.secretField("key", "", "old"); err != nil || got != "old" {
		t.Fatalf("keep existing: %q %v", got, err)
	}
}

func TestUninstall_noComposeFile(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	out := captureStdout(t, func() {
		if err := runCLI(t, "uninstall"); err != nil {
			t.Fatal(err)
		}
	})
	mustContain(t, out, "nothing to tear down")
	if e.calls(t) != "" {
		t.Error("engine must not run without compose file")
	}
}

func TestVersionAndExecute(t *testing.T) {
	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"version"})
		if err := Execute(); err != nil {
			t.Fatal(err)
		}
	})
	mustContain(t, out, "convergence version")
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
}

func mustLook(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// captureStdout runs f with os.Stdout redirected and returns what was written.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() { os.Stdout = old }()
		f()
	}()
	_ = w.Close()
	return <-done
}
