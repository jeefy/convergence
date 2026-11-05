package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/config"
)

type fakeRunner struct{ up []string }

func (f fakeRunner) RunningServices(context.Context, string) ([]string, error) { return f.up, nil }

func port(t *testing.T, s *httptest.Server) int {
	t.Helper()
	_, p, _ := net.SplitHostPort(strings.TrimPrefix(s.URL, "http://"))
	n, _ := strconv.Atoi(p)
	return n
}

func TestCheck_when_gatewayUpAndOthersDown(t *testing.T) {
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer gw.Close()
	standinPort, pinballPort = freePort(t), freePort(t)
	cfg := config.DefaultConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.GatewayPort = port(t, gw)

	ss := CheckWith(context.Background(), cfg, fakeRunner{up: []string{"agentgateway", "stand-in"}}, "")

	by := map[string]ServiceStatus{}
	for _, s := range ss {
		by[s.Name] = s
	}
	if g := by["agentgateway"]; !g.Healthy || !g.Running {
		t.Errorf("gateway should be up: %+v", g)
	}
	// stand-in container is "running" but its probe port is not listening.
	if s := by["stand-in"]; !s.Running || s.Healthy {
		t.Errorf("stand-in should be running but unhealthy: %+v", s)
	}
	if AllRequiredUp(ss) {
		t.Error("AllRequiredUp must be false")
	}
	if _, ok := by["copilot-api"]; ok {
		t.Error("copilot-api probed for non-copilot provider")
	}
}

func TestStandInBanner(t *testing.T) {
	down := []ServiceStatus{{Name: "stand-in"}}
	if b := StandInBanner(down, false); !strings.Contains(b, "REJECTED") {
		t.Errorf("fail-closed banner wrong: %q", b)
	}
	if b := StandInBanner(down, true); !strings.Contains(b, "UNSCRUBBED") {
		t.Errorf("fail-open banner wrong: %q", b)
	}
	if b := StandInBanner([]ServiceStatus{{Name: "stand-in", Running: true, Healthy: true}}, false); b != "" {
		t.Errorf("healthy stand-in must not warn: %q", b)
	}
}

func TestServerError_isUnhealthy(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer s.Close()
	cfg := config.DefaultConfig()
	cfg.GatewayPort = port(t, s)
	if ss := Check(context.Background(), cfg); ss[0].Healthy {
		t.Errorf("503 must be unhealthy: %+v", ss[0])
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}
