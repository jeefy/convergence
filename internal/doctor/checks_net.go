package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jeefy/convergence/internal/telemetry"
)

// CheckStandin verifies the metrics endpoint serves standin_* series and
// reports the configured mode.
func CheckStandin(ctx context.Context, e *Env) Result {
	const name = "stand-in metrics"
	smp, err := e.scrape(ctx, e.StandinURL)
	if err != nil {
		return Result{name, Fail, "cannot scrape " + e.StandinURL + "/metrics: " + shortErr(err), "run 'convergence start'; 'convergence logs stand-in'"}
	}
	n := 0
	for _, s := range smp {
		if strings.HasPrefix(s.Name, "standin_") {
			n++
		}
	}
	if n == 0 {
		return Result{name, Fail, "endpoint is up but serves no standin_* series (wrong service on this port?)", "check port 9102 and 'convergence logs stand-in'"}
	}
	mode := composeStandinMode(e.Dir)
	for _, s := range smp {
		if s.Name == "standin_requests_total" && s.Labels["mode"] != "" && mode == "" {
			mode = s.Labels["mode"]
		}
	}
	switch mode {
	case "enforce":
		return Result{name, Pass, fmt.Sprintf("%d standin_* series, mode=enforce", n), ""}
	case "":
		return Result{name, Warn, fmt.Sprintf("%d standin_* series, mode unknown", n), "check STANDIN_MODE in ~/.convergence/docker-compose.yaml"}
	default:
		return Result{name, Warn, fmt.Sprintf("%d standin_* series, mode=%s (secrets are not stripped unless mode=enforce)", n, mode), "set STANDIN_MODE: enforce and run 'convergence restart'"}
	}
}

// PinballDecision is the /check response.
type PinballDecision struct {
	Model  string `json:"model"`
	Tier   string `json:"tier"`
	Reason string `json:"reason"`
}

// PinballPrompts are the probe prompts: trivial, medium, hard.
var PinballPrompts = []struct{ Name, Text string }{
	{"trivial", "hi"},
	{"medium", "Write a Go function that parses a CSV file and returns the rows as structs, with unit tests."},
	{"hard", "Design a distributed consensus protocol for a security-critical microservice mesh, analyze its race conditions and prove its safety and liveness properties."},
}

// CheckPinball POSTs each probe prompt to pinball /check and shows the decision.
func CheckPinball(ctx context.Context, e *Env) []Result {
	out := make([]Result, 0, len(PinballPrompts))
	for _, p := range PinballPrompts {
		n := "pinball /check (" + p.Name + ")"
		body, _ := json.Marshal(map[string]any{
			"model":    "auto",
			"messages": []map[string]string{{"role": "user", "content": p.Text}},
		})
		code, raw, err := e.do(ctx, "POST", e.PinballURL+"/check", "application/json", string(body))
		if err != nil {
			out = append(out, Result{n, Fail, "request failed: " + shortErr(err), "run 'convergence start'; 'convergence logs pinball'"})
			continue
		}
		var d PinballDecision
		if code != 200 || json.Unmarshal(raw, &d) != nil || d.Model == "" || d.Tier == "" {
			out = append(out, Result{n, Fail, fmt.Sprintf("HTTP %d, unusable body: %s", code, trunc(string(raw), 120)), "check 'convergence logs pinball' and ~/.convergence/pinball.yaml"})
			continue
		}
		out = append(out, Result{n, Pass, fmt.Sprintf("tier=%s model=%s reason=%s", d.Tier, d.Model, d.Reason), ""})
	}
	return out
}

// CheckProvider probes the upstream provider where it can without credentials.
func CheckProvider(ctx context.Context, e *Env) Result {
	const name = "provider reachability"
	probe := func(url, label, hint string) Result {
		code, _, err := e.do(ctx, "GET", url, "", "")
		switch {
		case err != nil:
			return Result{name, Fail, label + " unreachable at " + url + ": " + shortErr(err), hint}
		case code != 200:
			return Result{name, Fail, fmt.Sprintf("%s returned HTTP %d at %s", label, code, url), hint}
		}
		return Result{name, Pass, label + " reachable at " + url, ""}
	}
	switch string(e.Cfg.Provider) {
	case "ollama":
		return probe(e.OllamaURL+"/api/tags", "ollama", "start ollama, or fix ollama_url in ~/.convergence/config.yaml (containers reach the host via host.docker.internal)")
	case "copilot":
		return probe(e.CopilotURL+"/v1/models", "copilot-api", "run 'convergence login copilot' and 'convergence logs copilot-api'")
	default:
		return Result{name, Info, "direct provider: API keys are only checked with --e2e", "run 'convergence doctor --e2e' to send one real request"}
	}
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// Counters is a snapshot of the three counters the e2e check compares.
type Counters struct{ Matches, Restored, Decisions float64 }

// Snapshot scrapes the counters; either endpoint failing is an error.
func Snapshot(ctx context.Context, e *Env) (Counters, error) {
	var c Counters
	si, err := e.scrape(ctx, e.StandinURL)
	if err != nil {
		return c, fmt.Errorf("stand-in metrics: %w", err)
	}
	pb, err := e.scrape(ctx, e.PinballURL)
	if err != nil {
		return c, fmt.Errorf("pinball metrics: %w", err)
	}
	c.Matches = telemetry.Sum(si, "standin_matches_total", nil)
	c.Restored = telemetry.Sum(si, "standin_restored_total", nil)
	c.Decisions = telemetry.Sum(pb, "pinball_decisions_total", nil)
	return c, nil
}
