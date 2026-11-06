package templates

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/models"
)

//go:embed assets/*
var assetsFS embed.FS

// Images pinned by this binary. `convergence upgrade` re-renders with these.
type Images struct {
	AgentGateway     string
	StandIn          string
	Pinball          string
	CopilotAPI       string
	OpenShellGateway string
}

var PinnedImages = Images{
	AgentGateway: "cr.agentgateway.dev/agentgateway:v1.6.0@sha256:482921556876a503ad3675b29223b1897b63a316983e46797ff99ebf83a2a6a2",
	StandIn:      "ghcr.io/jeefy/stand-in:0.4.1@sha256:c2216ae305f6dbff3f85e893ff720a177c712a69e8309a6e639afcb23fe05a2c",
	Pinball:      "ghcr.io/jeefy/pinball:0.7.3@sha256:01c79ee45faa6877c702bc5aacf889cdc77498d50196edc9943068d3d4d1f936",
	// v0.1.25 never existed on ghcr; v0.7.0 verified (entrypoint uses GH_TOKEN).
	CopilotAPI:       "ghcr.io/ericc-ch/copilot-api:v0.7.0@sha256:acf4f91931b78b7a4366e72447cfd170a3f3cf6388b9189b098c7236510fbb29",
	OpenShellGateway: "ghcr.io/nvidia/openshell/gateway:latest",
}

type HostPorts struct {
	GatewayStats    int
	StandinMetrics  int
	Pinball         int
	CopilotAPI      int
	OpenShell       int
	OpenShellHealth int
}

type RenderContext struct {
	Config *config.Config
	Images Images
	Ports  HostPorts
	// Models is the resolved tier/model mapping (internal/models).
	Models *models.Resolved
	// VolumeSuffix adds a shared SELinux relabel on Linux so rootless podman
	// (and docker on SELinux hosts) can read the bind-mounted configs.
	VolumeSuffix string
}

// File names inside the config dir.
const (
	ComposeFile = "docker-compose.yaml"
	EnvFile     = ".env"
)

// publicFiles hold no secrets and are bind-mounted into containers running
// as other uids, so they must be world-readable.
var publicFiles = []string{ComposeFile, "agentgateway.yaml", "pinball.yaml", "copilot-api-idle-timeout.js", "copilot-api-bunfig.toml"}

// VolumeSuffix is the bind-mount option suffix for this host.
func VolumeSuffix() string {
	if runtime.GOOS == "linux" {
		return ",z"
	}
	return ""
}

func ResolveImages(cfg *config.Config) Images {
	imgs := PinnedImages
	if cfg == nil {
		return imgs
	}
	if cfg.ImageAgentGateway != "" {
		imgs.AgentGateway = cfg.ImageAgentGateway
	}
	if cfg.ImageStandIn != "" {
		imgs.StandIn = cfg.ImageStandIn
	}
	if cfg.ImagePinball != "" {
		imgs.Pinball = cfg.ImagePinball
	}
	if cfg.ImageCopilotAPI != "" {
		imgs.CopilotAPI = cfg.ImageCopilotAPI
	}
	if cfg.ImageOpenShellGateway != "" {
		imgs.OpenShellGateway = cfg.ImageOpenShellGateway
	}
	return imgs
}

func newContext(cfg *config.Config, res *models.Resolved) RenderContext {
	return RenderContext{
		Config: cfg,
		Models: res,
		Images: ResolveImages(cfg),
		Ports: HostPorts{
			GatewayStats:    config.GatewayStatsPort,
			StandinMetrics:  config.StandinMetricsPort,
			Pinball:         config.PinballPort,
			CopilotAPI:      config.CopilotAPIPort,
			OpenShell:       config.OpenShellPort,
			OpenShellHealth: config.OpenShellHealthPort,
		},
		VolumeSuffix: VolumeSuffix(),
	}
}

// RenderAssets writes the compose stack into targetDir: non-secret files 0644
// and the secret-bearing .env 0600 (read only by the compose CLI on the host).
func RenderAssets(cfg *config.Config, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating target dir: %w", err)
	}
	// MkdirAll keeps the mode of an existing dir (older versions used 0700).
	if err := os.Chmod(targetDir, 0o755); err != nil {
		return fmt.Errorf("chmod target dir: %w", err)
	}

	res, err := models.ForConfig(cfg, targetDir)
	if err != nil {
		return err
	}
	ctx := newContext(cfg, res)
	filesToRender := append([]string{}, publicFiles...)
	if cfg.Sandbox == config.SandboxOpenShell {
		filesToRender = append(filesToRender, "openshell.toml")
	}

	for _, file := range filesToRender {
		data, err := assetsFS.ReadFile("assets/" + file + ".tmpl")
		if err != nil {
			return fmt.Errorf("reading template %s: %w", file, err)
		}
		tmpl, err := template.New(file).Funcs(template.FuncMap{"providerRef": providerRef}).Option("missingkey=error").Parse(string(data))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", file, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, ctx); err != nil {
			return fmt.Errorf("executing template %s: %w", file, err)
		}
		if err := writeFile(filepath.Join(targetDir, file), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return writeFile(filepath.Join(targetDir, EnvFile), RenderEnv(cfg), 0o600)
}

// RenderEnv returns the compose .env content carrying every secret.
func RenderEnv(cfg *config.Config) []byte {
	var b bytes.Buffer
	b.WriteString("# Secrets for the convergence compose stack. Keep this file 0600.\n")
	env := [][2]string{{"STANDIN_SALT", cfg.StandinSalt}}
	switch cfg.Provider {
	case config.ProviderCopilot:
		env = append(env, [2]string{"GH_TOKEN", cfg.CopilotToken})
	case config.ProviderDirect:
		env = append(env,
			[2]string{"OPENAI_API_KEY", cfg.OpenAIKey},
			[2]string{"ANTHROPIC_API_KEY", cfg.AnthropicKey})
	case config.ProviderOllama:
	}
	for _, kv := range env {
		// Single quotes: compose takes the value literally (no $ expansion).
		fmt.Fprintf(&b, "%s='%s'\n", kv[0], kv[1])
	}
	return b.Bytes()
}

// writeFile writes data and enforces perm even if the file already existed.
func writeFile(path string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// providerRef maps an internal/models upstream family to the provider name
// declared in agentgateway.yaml.tmpl.
func providerRef(family string) (string, error) {
	switch family {
	case models.UpstreamCopilot:
		return "copilot-completions", nil
	case models.UpstreamOllama:
		return "ollama-local", nil
	case models.UpstreamOpenAI:
		return "openai-direct", nil
	case models.UpstreamAnthropic:
		return "anthropic-direct", nil
	}
	return "", fmt.Errorf("unknown upstream family %q", family)
}
