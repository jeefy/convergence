package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/jeefy/convergence/internal/config"
)

var execLookPath = exec.LookPath

type StatusResult struct {
	Type      config.SandboxType `json:"type"`
	Available bool               `json:"available"`
	Message   string             `json:"message"`
	Endpoint  string             `json:"endpoint,omitempty"`
}

type Driver interface {
	Type() config.SandboxType
	Status(ctx context.Context) (StatusResult, error)
	InferenceURL() string
	RunCommand(agent string, args []string) ([]string, []string, error)
}

func GetDriver(sbx config.SandboxType, cfg *config.Config) (Driver, error) {
	switch sbx {
	case "", config.SandboxNone:
		return &noneDriver{}, nil
	case config.SandboxOpenShell:
		return &openShellDriver{cfg: cfg}, nil
	case config.SandboxNono:
		return &nonoDriver{cfg: cfg}, nil
	case config.SandboxDockerSbx:
		return &dockerSbxDriver{cfg: cfg}, nil
	case config.SandboxK8sAgent:
		return &k8sAgentDriver{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported sandbox driver: %s", sbx)
	}
}

type noneDriver struct{}

func (d *noneDriver) Type() config.SandboxType {
	return config.SandboxNone
}

func (d *noneDriver) Status(ctx context.Context) (StatusResult, error) {
	return StatusResult{
		Type:      config.SandboxNone,
		Available: false,
		Message:   "no sandbox configured",
	}, nil
}

func (d *noneDriver) InferenceURL() string {
	return ""
}

func (d *noneDriver) RunCommand(agent string, args []string) ([]string, []string, error) {
	cmd := append([]string{agent}, args...)
	return cmd, nil, nil
}

type openShellDriver struct {
	cfg *config.Config
}

func (d *openShellDriver) Type() config.SandboxType {
	return config.SandboxOpenShell
}

func (d *openShellDriver) Status(ctx context.Context) (StatusResult, error) {
	host := "127.0.0.1"
	if d.cfg != nil && d.cfg.HealthHost() != "" {
		host = d.cfg.HealthHost()
	}
	healthURL := fmt.Sprintf("http://%s:%d/healthz", host, config.OpenShellHealthPort)
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return StatusResult{
			Type:      config.SandboxOpenShell,
			Available: false,
			Message:   err.Error(),
		}, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return StatusResult{
			Type:      config.SandboxOpenShell,
			Available: false,
			Message:   fmt.Sprintf("gateway unreachable at %s: %v", healthURL, err),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return StatusResult{
			Type:      config.SandboxOpenShell,
			Available: false,
			Message:   fmt.Sprintf("gateway unhealthy: HTTP %d", resp.StatusCode),
		}, nil
	}
	return StatusResult{
		Type:      config.SandboxOpenShell,
		Available: true,
		Message:   "gateway running and healthy",
		Endpoint:  fmt.Sprintf("http://%s:%d", host, config.OpenShellPort),
	}, nil
}

func (d *openShellDriver) InferenceURL() string {
	return "https://inference.local"
}

func (d *openShellDriver) RunCommand(agent string, args []string) ([]string, []string, error) {
	cmd := append([]string{"openshell", "sandbox", "create", "--", agent}, args...)
	return cmd, nil, nil
}

type dockerSbxDriver struct {
	cfg *config.Config
}

func (d *dockerSbxDriver) Type() config.SandboxType {
	return config.SandboxDockerSbx
}

func (d *dockerSbxDriver) Status(ctx context.Context) (StatusResult, error) {
	return StatusResult{
		Type:      config.SandboxDockerSbx,
		Available: true,
		Message:   "docker sandboxes (sbx) host driver configured",
		Endpoint:  "docker-sbx",
	}, nil
}

func (d *dockerSbxDriver) InferenceURL() string {
	return "http://host.docker.internal:3000/v1"
}

func (d *dockerSbxDriver) RunCommand(agent string, args []string) ([]string, []string, error) {
	cmd := append([]string{"sbx", "run", "--", agent}, args...)
	env := []string{
		"OPENAI_BASE_URL=" + d.InferenceURL(),
	}
	return cmd, env, nil
}

type k8sAgentDriver struct {
	cfg *config.Config
}

func (d *k8sAgentDriver) Type() config.SandboxType {
	return config.SandboxK8sAgent
}

func (d *k8sAgentDriver) Status(ctx context.Context) (StatusResult, error) {
	return StatusResult{
		Type:      config.SandboxK8sAgent,
		Available: true,
		Message:   "kubernetes agents.x-k8s.io sandbox driver configured",
		Endpoint:  "k8s-agent",
	}, nil
}

func (d *k8sAgentDriver) InferenceURL() string {
	return "http://agentgateway.convergence.svc:3000/v1"
}

func (d *k8sAgentDriver) RunCommand(agent string, args []string) ([]string, []string, error) {
	return nil, nil, fmt.Errorf("k8s-agent workloads must be launched via Kubernetes Custom Resource")
}

type nonoDriver struct {
	cfg *config.Config
}

func (d *nonoDriver) Type() config.SandboxType {
	return config.SandboxNono
}

func (d *nonoDriver) Status(ctx context.Context) (StatusResult, error) {
	path, err := execLookPath("nono")
	if err != nil {
		return StatusResult{
			Type:      config.SandboxNono,
			Available: false,
			Message:   "'nono' binary not found on PATH (install via 'curl -fsSL https://nono.sh/install.sh | sh' or 'brew install nono')",
		}, nil
	}
	return StatusResult{
		Type:      config.SandboxNono,
		Available: true,
		Message:   fmt.Sprintf("nono CLI available at %s", path),
		Endpoint:  path,
	}, nil
}

func (d *nonoDriver) InferenceURL() string {
	host := "127.0.0.1"
	port := config.DefaultGatewayPort
	if d.cfg != nil {
		if d.cfg.HealthHost() != "" {
			host = d.cfg.HealthHost()
		}
		if d.cfg.GatewayPort > 0 {
			port = d.cfg.GatewayPort
		}
	}
	return fmt.Sprintf("http://%s:%d/v1", host, port)
}

func (d *nonoDriver) RunCommand(agent string, args []string) ([]string, []string, error) {
	cmd := append([]string{"nono", "run", "--allow-cwd", "--", agent}, args...)
	base := d.InferenceURL()
	env := []string{
		"OPENAI_BASE_URL=" + base,
		"ANTHROPIC_BASE_URL=" + fmt.Sprintf("http://127.0.0.1:%d", config.DefaultGatewayPort),
		"OPENAI_API_KEY=unused",
		"ANTHROPIC_API_KEY=unused",
	}
	return cmd, env, nil
}
