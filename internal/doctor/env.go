package doctor

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/telemetry"
)

// ServiceStatus mirrors the health package's per-service status.
type ServiceStatus struct {
	Name      string
	Container string
	Running   bool
	Healthy   bool
	Detail    string
}

// ServiceChecker reports per-service state.
type ServiceChecker interface {
	Check(ctx context.Context) []ServiceStatus
}

// Env carries everything the checks read. Tests point the URLs at httptest servers.
type Env struct {
	Cfg        *config.Config
	Dir        string // ~/.convergence
	FailOpen   bool   // fail_open from config.yaml (missing = false)
	HTTP       *http.Client
	GatewayURL string // http://host:port
	StandinURL string
	PinballURL string
	GWStatsURL string
	OllamaURL  string // host-reachable
	CopilotURL string
	Services   ServiceChecker
	// Run executes an external command (container engine probes).
	Run func(ctx context.Context, name string, args ...string) error
}

// NewEnv builds an Env for a loaded config.
func NewEnv(cfg *config.Config, dir string) *Env {
	host := cfg.HealthHost()
	u := func(p int) string { return "http://" + net.JoinHostPort(host, strconv.Itoa(p)) }
	return &Env{
		Cfg: cfg, Dir: dir,
		HTTP:       &http.Client{},
		GatewayURL: u(cfg.GatewayPort),
		StandinURL: u(config.StandinMetricsPort),
		PinballURL: u(config.PinballPort),
		GWStatsURL: u(config.GatewayStatsPort),
		OllamaURL:  hostReachable(cfg.OllamaURL),
		CopilotURL: u(config.CopilotAPIPort),
	}
}

// hostReachable maps container-only names to loopback for probing from the host.
func hostReachable(raw string) string {
	return strings.Replace(raw, "host.docker.internal", "127.0.0.1", 1)
}

// ReadFailOpen reads the optional `fail_open` key; missing or unreadable = false.
func ReadFailOpen(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return false
	}
	var raw struct {
		FailOpen bool `yaml:"fail_open"`
	}
	return yaml.Unmarshal(data, &raw) == nil && raw.FailOpen
}

// getJSONStatus issues a GET and returns status and body (capped).
func (e *Env) do(ctx context.Context, method, url, ctype, body string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for len(buf) < 1<<20 {
		n, rerr := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if rerr != nil {
			break
		}
	}
	return resp.StatusCode, buf, nil
}

func (e *Env) scrape(ctx context.Context, base string) ([]telemetry.Sample, error) {
	return telemetry.Scrape(ctx, e.HTTP, base+"/metrics")
}

func shortErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && strings.Contains(s, "connection refused") {
		return "connection refused"
	}
	return s
}

func dial(ctx context.Context, addr string) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return c.Close()
}

func portAddr(host string, port int) string { return net.JoinHostPort(host, strconv.Itoa(port)) }
