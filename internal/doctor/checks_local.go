package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jeefy/convergence/internal/config"
)

// CheckConfig verifies config.yaml exists and parses and that .env is 0600.
// The loaded config is nil when the config is unusable.
func CheckConfig(dir string) (Result, *config.Config) {
	const name = "config"
	cfgPath := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return Result{name, Fail, fmt.Sprintf("cannot read %s: %v", cfgPath, err), "run 'convergence init'"}, nil
	}
	var probe config.Config
	if err := yaml.Unmarshal(data, &probe); err != nil {
		return Result{name, Fail, fmt.Sprintf("%s does not parse: %v", cfgPath, err), "fix the YAML or re-run 'convergence init'"}, nil
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return Result{name, Fail, err.Error(), "fix the YAML or re-run 'convergence init'"}, nil
	}
	envPath := filepath.Join(dir, ".env")
	st, err := os.Stat(envPath)
	switch {
	case err != nil:
		return Result{name, Fail, fmt.Sprintf("config parses but %s is missing: %v", envPath, err), "run 'convergence init' to regenerate secrets"}, cfg
	case st.Mode().Perm() != 0o600:
		return Result{name, Fail, fmt.Sprintf("%s has mode %04o, want 0600", envPath, st.Mode().Perm()), "chmod 600 " + envPath}, cfg
	}
	return Result{name, Pass, fmt.Sprintf("%s parses (provider=%s, engine=%s); .env is 0600", cfgPath, cfg.Provider, cfg.Engine), ""}, cfg
}

// CheckEngine verifies the container engine answers and compose is available.
func CheckEngine(ctx context.Context, e *Env) Result {
	const name = "container engine"
	bin := e.Cfg.Engine
	if bin == "" {
		bin = "docker"
	}
	if err := e.Run(ctx, bin, "info"); err != nil {
		return Result{name, Fail, fmt.Sprintf("'%s info' failed: %v", bin, err), "start the " + bin + " daemon/socket (e.g. 'systemctl --user start podman.socket')"}
	}
	if err := e.Run(ctx, bin, "compose", "version"); err != nil {
		return Result{name, Fail, fmt.Sprintf("'%s compose version' failed: %v", bin, err), "install the compose plugin (docker-compose-plugin or podman-compose)"}
	}
	return Result{name, Pass, bin + " reachable, compose available", ""}
}

// CheckServices reports each service's running/healthy state via the health package.
func CheckServices(ctx context.Context, e *Env) []Result {
	if e.Services == nil {
		return []Result{{"services", Warn, "no service health checker wired", "update convergence"}}
	}
	sts := e.Services.Check(ctx)
	if len(sts) == 0 {
		return []Result{{"services", Fail, "no services reported", "run 'convergence start'"}}
	}
	out := make([]Result, 0, len(sts))
	for _, s := range sts {
		n := "service " + s.Name
		switch {
		case !s.Running:
			out = append(out, Result{n, Fail, orDefault(s.Detail, "not running"), "run 'convergence start' then 'convergence logs " + s.Name + "'"})
		case !s.Healthy:
			out = append(out, Result{n, Fail, orDefault(s.Detail, "running but unhealthy"), "inspect with 'convergence logs " + s.Name + "'"})
		default:
			out = append(out, Result{n, Pass, orDefault(s.Detail, "running and healthy"), ""})
		}
	}
	return out
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// CheckPorts verifies every published port accepts connections and that the
// bind address is loopback-only.
func CheckPorts(ctx context.Context, e *Env) []Result {
	var out []Result
	bind := e.Cfg.BindAddress
	if bind != "127.0.0.1" {
		out = append(out, Result{"bind address", Warn, fmt.Sprintf("bind_address=%q exposes the gateway beyond loopback", bind),
			"set bind_address: 127.0.0.1 in ~/.convergence/config.yaml and run 'convergence restart'"})
	} else {
		out = append(out, Result{"bind address", Pass, "published ports bind to 127.0.0.1 only", ""})
	}
	host := e.Cfg.HealthHost()
	for _, p := range e.Cfg.PublishedPorts() {
		n := fmt.Sprintf("port %d (%s)", p.Port, p.Name)
		if err := dial(ctx, portAddr(host, p.Port)); err != nil {
			out = append(out, Result{n, Fail, "not listening on " + portAddr(host, p.Port) + ": " + shortErr(err), "run 'convergence start'"})
			continue
		}
		out = append(out, Result{n, Pass, "listening on " + portAddr(host, p.Port), ""})
	}
	return out
}

// composeStandinMode reads STANDIN_MODE from the rendered compose file.
func composeStandinMode(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "docker-compose.yaml"))
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(data), "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "STANDIN_MODE:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "STANDIN_MODE:")), `"'`)
		}
	}
	return ""
}

// standinFailOpen reports whether the rendered gateway config has Stand-In
// failing open. ok=false when it cannot be determined.
func standinFailOpen(dir string) (open, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, "agentgateway.yaml"))
	if err != nil {
		return false, false
	}
	var y struct {
		Gateways map[string]struct {
			ExtProc struct {
				Conditional []struct {
					Host        string `yaml:"host"`
					FailureMode string `yaml:"failureMode"`
				} `yaml:"conditional"`
			} `yaml:"extProc"`
		} `yaml:"gateways"`
	}
	if yaml.Unmarshal(data, &y) != nil {
		return false, false
	}
	for _, g := range y.Gateways {
		for _, c := range g.ExtProc.Conditional {
			if strings.HasPrefix(c.Host, "stand-in") {
				return c.FailureMode == "failOpen", true
			}
		}
	}
	return false, false
}

// CheckFailMode reports whether Stand-In is fail-closed or fail-open.
func CheckFailMode(e *Env) Result {
	const name = "stand-in fail mode"
	rendered, ok := standinFailOpen(e.Dir)
	hint := "set fail_open: false in ~/.convergence/config.yaml and run 'convergence restart' (fail-closed blocks traffic when Stand-In is down)"
	switch {
	case e.FailOpen || (ok && rendered):
		src := "config fail_open: true"
		if !e.FailOpen {
			src = "rendered agentgateway.yaml has failureMode: failOpen for stand-in (config fail_open is false/missing)"
		}
		return Result{name, Warn, "FAIL-OPEN: if Stand-In is down, requests reach the provider WITH raw secrets (" + src + ")", hint}
	default:
		return Result{name, Pass, "fail-closed: requests are blocked when Stand-In is unavailable", ""}
	}
}
