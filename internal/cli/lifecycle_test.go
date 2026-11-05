package cli

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeefy/convergence/internal/config"
)

func TestStart_when_notInitialized(t *testing.T) {
	newFakeEnv(t, "docker")
	err := runCLI(t, "start")
	if err == nil || !strings.Contains(err.Error(), "convergence init") {
		t.Fatalf("got %v", err)
	}
}

func TestStart_when_copilotWithoutToken(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.Provider = config.ProviderCopilot })
	err := runCLI(t, "start")
	if err == nil || !strings.Contains(err.Error(), "convergence login copilot") {
		t.Fatalf("got %v", err)
	}
	if e.calls(t) != "" {
		t.Error("engine must not run before the secret check")
	}
}

func TestStart_when_unknownProvider(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, func(c *config.Config) { c.Provider = "bogus" })
	if err := runCLI(t, "start"); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("got %v", err)
	}
}

func TestStart_when_portInUse(t *testing.T) {
	e := newFakeEnv(t, "docker")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := portOf(t, ln.Addr())
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
	t.Setenv("FAKE_RUNNING", "")

	err = runCLI(t, "start")

	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(port)) || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected port named in error, got %v", err)
	}
	if strings.Contains(e.calls(t), "up -d") {
		t.Error("must not start compose when port is taken")
	}
}

func TestStart_when_engineMissing(t *testing.T) {
	e := newFakeEnv(t) // no binaries
	e.saveCfg(t, nil)
	err := runCLI(t, "start")
	if err == nil || !strings.Contains(err.Error(), "docs.docker.com/get-docker") {
		t.Fatalf("want install hint, got %v", err)
	}
}

func TestStart_when_composeUpFails(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	t.Setenv("FAKE_FAIL", "up -d")
	err := runCLI(t, "start")
	if err == nil || !strings.Contains(err.Error(), "compose up -d failed") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("got %v", err)
	}
}

func TestStart_when_healthNeverReady(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := freePort(t)
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
	old := healthTimeout
	healthTimeout = 300 * time.Millisecond
	t.Cleanup(func() { healthTimeout = old })

	err := runCLI(t, "start")

	if err == nil || !strings.Contains(err.Error(), "not healthy") || !strings.Contains(err.Error(), "convergence logs agentgateway") {
		t.Fatalf("got %v", err)
	}
}

func TestStart_when_healthy(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := healthyGateway(t)
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
	if err := runCLI(t, "start"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, e.calls(t), "up -d")
	if _, err := os.Stat(filepath.Join(e.dir, "docker-compose.yaml")); err != nil {
		t.Errorf("assets not rendered: %v", err)
	}
}

func TestStart_when_ollamaUnreachable(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := healthyGateway(t)
	dead := freePort(t)
	e.saveCfg(t, func(c *config.Config) {
		c.GatewayPort = port
		c.Provider = config.ProviderOllama
		c.OllamaURL = "http://127.0.0.1:" + strconv.Itoa(dead)
	})
	if err := runCLI(t, "start"); err != nil {
		t.Fatalf("non-strict must only warn: %v", err)
	}
	err := runCLI(t, "start", "--strict")
	if err == nil || !strings.Contains(err.Error(), "cannot check Ollama models") {
		t.Fatalf("strict: got %v", err)
	}
}

func TestStop_when_composeDownFails(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	t.Setenv("FAKE_FAIL", "down")
	err := runCLI(t, "stop")
	if err == nil || !strings.Contains(err.Error(), "compose down failed") {
		t.Fatalf("got %v", err)
	}
}

func TestStop_when_ok(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	if err := runCLI(t, "stop"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, e.calls(t), "compose -f "+filepath.Join(e.dir, "docker-compose.yaml")+" down")
}

func TestRestart_when_notInitialized(t *testing.T) {
	newFakeEnv(t, "docker")
	if err := runCLI(t, "restart"); err == nil {
		t.Fatal("want error")
	}
}

func TestRestart_when_stopFails_doesNotStart(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	t.Setenv("FAKE_FAIL", "down")
	if err := runCLI(t, "restart"); err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(e.calls(t), "up -d") {
		t.Error("up must not run after failed down")
	}
}

func TestRestart_when_ok_downThenUp(t *testing.T) {
	e := newFakeEnv(t, "docker")
	port := healthyGateway(t)
	e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
	if err := runCLI(t, "restart"); err != nil {
		t.Fatal(err)
	}
	c := e.calls(t)
	if !strings.Contains(c, " down") || strings.Index(c, " down") > strings.Index(c, "up -d") {
		t.Errorf("order wrong:\n%s", c)
	}
}

func TestUninstall_when_noPurge_keepsConfig(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	if err := os.WriteFile(filepath.Join(e.dir, "docker-compose.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCLI(t, "uninstall"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, e.calls(t), "down -v --remove-orphans")
	if _, err := os.Stat(e.dir); err != nil {
		t.Error("config dir must be kept")
	}
}

func TestUninstall_when_composeDownFails_keepsEverything(t *testing.T) {
	e := newFakeEnv(t, "docker")
	e.saveCfg(t, nil)
	if err := os.WriteFile(filepath.Join(e.dir, "docker-compose.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_FAIL", "down")
	err := runCLI(t, "uninstall", "--purge", "--yes")
	if err == nil || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(e.dir); err != nil {
		t.Error("must not purge after down failure")
	}
}

func TestUninstall_purge(t *testing.T) {
	for _, tc := range []struct {
		name, stdin string
		args        []string
		wantGone    bool
	}{
		{"yes flag", "", []string{"--purge", "--yes"}, true},
		{"confirmed", "y\n", []string{"--purge"}, true},
		{"declined", "n\n", []string{"--purge"}, false},
		{"eof is not consent", "", []string{"--purge"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newFakeEnv(t, "docker")
			e.saveCfg(t, nil)
			sibling := filepath.Join(e.home, "keep.txt")
			if err := os.WriteFile(sibling, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			withStdin(t, tc.stdin)

			if err := runCLI(t, append([]string{"uninstall"}, tc.args...)...); err != nil {
				t.Fatal(err)
			}

			_, err := os.Stat(e.dir)
			if gone := os.IsNotExist(err); gone != tc.wantGone {
				t.Errorf("config dir gone=%v want %v", gone, tc.wantGone)
			}
			if _, err := os.Stat(sibling); err != nil {
				t.Error("purge touched files outside ~/.convergence")
			}
		})
	}
}

func TestUpgrade_errorsAtEachStep(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		newFakeEnv(t, "docker")
		if err := runCLI(t, "upgrade"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("pull fails", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		t.Setenv("FAKE_FAIL", "pull")
		err := runCLI(t, "upgrade")
		if err == nil || !strings.Contains(err.Error(), "compose pull failed") {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(e.calls(t), "up -d") {
			t.Error("up after failed pull")
		}
	})
	t.Run("up fails", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		t.Setenv("FAKE_FAIL", "up -d")
		if err := runCLI(t, "upgrade"); err == nil || !strings.Contains(err.Error(), "up -d") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("render fails", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		// a directory where a rendered file must go
		if err := os.Mkdir(filepath.Join(e.dir, "docker-compose.yaml"), 0o700); err != nil {
			t.Fatal(err)
		}
		err := runCLI(t, "upgrade")
		if err == nil || !strings.Contains(err.Error(), "rendering assets") {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(e.calls(t), "pull") {
			t.Error("pull after render failure")
		}
	})
	t.Run("health fails", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		port := freePort(t)
		e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
		old := healthTimeout
		healthTimeout = 200 * time.Millisecond
		t.Cleanup(func() { healthTimeout = old })
		if err := runCLI(t, "upgrade"); err == nil || !strings.Contains(err.Error(), "not healthy") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("success order render pull up", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		port := healthyGateway(t)
		e.saveCfg(t, func(c *config.Config) { c.GatewayPort = port })
		if err := runCLI(t, "upgrade"); err != nil {
			t.Fatal(err)
		}
		c := e.calls(t)
		if strings.Index(c, "pull") > strings.Index(c, "up -d") || !strings.Contains(c, "pull") {
			t.Errorf("order:\n%s", c)
		}
	})
}

func TestLogs(t *testing.T) {
	t.Run("invalid service rejected", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		err := runCLI(t, "logs", "postgres")
		if err == nil || !strings.Contains(err.Error(), "postgres") {
			t.Fatalf("got %v", err)
		}
		if e.calls(t) != "" {
			t.Error("engine ran for invalid service")
		}
	})
	t.Run("follow and service passed", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		if err := runCLI(t, "logs", "-f", "pinball", "stand-in"); err != nil {
			t.Fatal(err)
		}
		mustContain(t, e.calls(t), "logs -f pinball stand-in")
	})
	t.Run("no follow", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		if err := runCLI(t, "logs"); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(e.calls(t), "-f pinball") || strings.Contains(e.calls(t), "logs -f") {
			t.Errorf("unexpected follow: %s", e.calls(t))
		}
	})
	t.Run("compose failure surfaced", func(t *testing.T) {
		e := newFakeEnv(t, "docker")
		e.saveCfg(t, nil)
		t.Setenv("FAKE_FAIL", "logs")
		if err := runCLI(t, "logs"); err == nil || !strings.Contains(err.Error(), "compose logs failed") {
			t.Fatalf("got %v", err)
		}
	})
}
