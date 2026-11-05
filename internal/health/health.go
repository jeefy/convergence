// Package health probes each local-stack service separately: container state
// from the compose engine plus an HTTP probe over the published loopback port.
// internal/doctor imports Check; keep its signature stable.
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jeefy/convergence/internal/config"
)

// ServiceStatus is the state of one compose service.
type ServiceStatus struct {
	Name      string `json:"name"`      // compose service name
	Container string `json:"container"` // "running" | "not running"
	Running   bool   `json:"running"`
	Healthy   bool   `json:"healthy"` // HTTP probe succeeded
	Detail    string `json:"detail"`  // probe URL result or error
	Required  bool   `json:"required"`
}

// Runner lists running compose service names (engine.ComposeEngine satisfies it).
type Runner interface {
	RunningServices(ctx context.Context, dir string) ([]string, error)
}

type probe struct {
	name string
	port func(*config.Config) int
	path string
}

// Fixed published ports; vars so tests can point them at closed/test ports.
var (
	standinPort = config.StandinMetricsPort
	pinballPort = config.PinballPort
	copilotPort = config.CopilotAPIPort
)

func probes(cfg *config.Config) []probe {
	ps := []probe{
		{"agentgateway", func(c *config.Config) int { return c.GatewayPort }, "/v1/models"},
		{"stand-in", func(*config.Config) int { return standinPort }, "/metrics"},
		{"pinball", func(*config.Config) int { return pinballPort }, "/healthz"},
	}
	if cfg.Provider == config.ProviderCopilot {
		ps = append(ps, probe{"copilot-api", func(*config.Config) int { return copilotPort }, "/"})
	}
	return ps
}

// Check probes every service of cfg's stack over loopback HTTP only (no
// container engine needed). Running mirrors Healthy unless RunningFrom is used.
func Check(ctx context.Context, cfg *config.Config) []ServiceStatus {
	return CheckWith(ctx, cfg, nil, "")
}

// CheckWith additionally asks the engine for container state when r != nil.
func CheckWith(ctx context.Context, cfg *config.Config, r Runner, dir string) []ServiceStatus {
	var running []string
	var runErr error
	if r != nil {
		running, runErr = r.RunningServices(ctx, dir)
	}
	var out []ServiceStatus
	for _, p := range probes(cfg) {
		st := ServiceStatus{Name: p.name, Container: p.name, Required: true}
		url := fmt.Sprintf("http://%s%s", net.JoinHostPort(cfg.HealthHost(), strconv.Itoa(p.port(cfg))), p.path)
		code, err := get(ctx, url)
		switch {
		case err != nil:
			st.Detail = err.Error()
		case code >= 500 || code == 0:
			st.Detail = fmt.Sprintf("%s returned HTTP %d", url, code)
		default:
			// copilot-api answers 200 on "/"; any non-5xx proves the process serves.
			st.Healthy = true
			st.Detail = fmt.Sprintf("%s HTTP %d", url, code)
		}
		switch {
		case r == nil:
			st.Running = st.Healthy
		case runErr != nil:
			st.Running = st.Healthy
		default:
			st.Running = slices.Contains(running, p.name)
		}
		st.Container = "not running"
		if st.Running {
			st.Container = "running"
		}
		out = append(out, st)
	}
	return out
}

func get(ctx context.Context, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body
	return resp.StatusCode, nil
}

// AllRequiredUp reports whether every required service is running and healthy.
func AllRequiredUp(ss []ServiceStatus) bool {
	for _, s := range ss {
		if s.Required && (!s.Running || !s.Healthy) {
			return false
		}
	}
	return true
}

// StandInBanner returns a loud multi-line warning when stand-in is not
// healthy, or "" when scrubbing is active.
func StandInBanner(ss []ServiceStatus, failOpen bool) string {
	for _, s := range ss {
		if s.Name != "stand-in" || (s.Running && s.Healthy) {
			continue
		}
		consequence := "requests through the gateway will be REJECTED (fail-closed)."
		if failOpen {
			consequence = "requests pass through UNSCRUBBED (fail_open is enabled): secrets reach the upstream!"
		}
		return "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n" +
			"!! WARNING: Stand-In is unhealthy: secret scrubbing is NOT active.\n" +
			"!! " + consequence + "\n" +
			"!! Inspect with: convergence logs stand-in\n" +
			"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n"
	}
	return ""
}
