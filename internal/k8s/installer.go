package k8s

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/models"
)

//go:embed assets/*
var k8sAssets embed.FS

type ClusterOptions struct {
	Namespace    string
	Provider     string
	CopilotToken string
	OpenAIKey    string
	AnthropicKey string
	StandinSalt  string
	OptimizerDSN string // when set, renders the optimizer CronJob + overlay PVC
	OptSchedule  string
	FailOpen     bool        // Stand-In ext_proc failureMode: failOpen (UNSCRUBBED passthrough) vs failClosed
	OTel         config.OTel // optional OTLP trace export
	// RequestLogDB is an external Postgres DSN; RequestLogPostgres renders one in-cluster.
	// Either enables agentgateway's request log (llm: full) and the optimizer CronJob.
	RequestLogDB         string
	RequestLogPostgres   bool
	RequestLogPGPassword string // generated when empty; reuse the existing one on re-install (PVC keeps the old one)
	RequestLogPGSize     string
	OpenAIBaseURL        string // testing/self-hosted OpenAI-compatible endpoint for provider=direct
	StandinPatternsFile  string // optional patterns.yaml; rendered into ConfigMap stand-in-patterns (replaces the image default)
	ModelsFile           string // optional models.yaml override (same schema as ~/.convergence/models.yaml)
	IngressHost          string // when set, renders an Ingress resource pointing to agentgateway:3000
	EnablePDB            bool   // when true, renders PodDisruptionBudgets (minAvailable: 1) for gateway, stand-in, and pinball
	PassthroughSecret    string // existing Secret whose keys are the passthrough api_key_env names
	DryRun               bool

	ImageAgentGateway string
	ImageStandIn      string
	// ExtProcTimeout overrides the ext_proc requestTimeout (default 5s).
	ExtProcTimeout  string
	ImagePinball    string
	ImageCopilotAPI string
	// StandinCPURequest/Limit override Stand-In's CPU (defaults 50m/500m).
	StandinCPURequest string
	StandinCPULimit   string
}

// RequestLogEnabled reports whether agentgateway writes the Postgres request log.
func (o *ClusterOptions) RequestLogEnabled() bool { return o.RequestLogDB != "" }

func RenderManifests(opts *ClusterOptions) (string, error) {
	if err := opts.OTel.Validate(); err != nil {
		return "", err
	}
	if err := config.ValidateExtProcTimeout(opts.ExtProcTimeout); err != nil {
		return "", err
	}
	for _, q := range []string{opts.StandinCPURequest, opts.StandinCPULimit} {
		if q != "" && !cpuQuantity.MatchString(q) {
			return "", fmt.Errorf("invalid stand-in CPU quantity %q (want e.g. 200m or 2)", q)
		}
	}
	switch opts.Provider {
	case "copilot", "direct":
	default:
		return "", fmt.Errorf("unsupported provider %q: must be copilot or direct", opts.Provider)
	}
	tmplData, err := k8sAssets.ReadFile("assets/manifests.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("reading k8s template: %w", err)
	}

	if opts.StandinSalt == "" {
		opts.StandinSalt, _ = config.GenerateSalt()
	}
	if opts.PassthroughSecret == "" {
		opts.PassthroughSecret = "convergence-passthrough"
	}
	if opts.RequestLogDB != "" && opts.RequestLogPostgres {
		return "", fmt.Errorf("--request-log-db and --request-log-postgres are mutually exclusive")
	}
	if opts.RequestLogPostgres {
		if opts.RequestLogPGPassword == "" {
			opts.RequestLogPGPassword, _ = config.GenerateSalt()
		}
		if opts.RequestLogPGSize == "" {
			opts.RequestLogPGSize = "5Gi"
		}
		opts.RequestLogDB = fmt.Sprintf("postgres://convergence:%s@request-log-postgres.%s.svc.cluster.local:5432/agentgateway?sslmode=disable", opts.RequestLogPGPassword, opts.Namespace)
	}
	if opts.RequestLogDB != "" {
		opts.OptimizerDSN = opts.RequestLogDB
	}
	if opts.OptSchedule == "" {
		opts.OptSchedule = "0 3 * * 0" // weekly Sunday 03:00
	}

	var override *models.Overrides
	var pinball *models.Pinball
	var passthrough []models.Passthrough
	if opts.ModelsFile != "" {
		raw, err := os.ReadFile(opts.ModelsFile)
		if err != nil {
			return "", fmt.Errorf("reading models file: %w", err)
		}
		o, err := models.ParseOverrides(raw)
		if err != nil {
			return "", err
		}
		override = &o
		if pinball, err = models.ParsePinball(raw); err != nil {
			return "", err
		}
		if passthrough, err = models.ParsePassthrough(raw); err != nil {
			return "", err
		}
	}
	res, err := models.Resolve(config.ProviderType(opts.Provider), override)
	if err != nil {
		return "", err
	}
	res.Passthrough = passthrough
	for _, p := range passthrough {
		if (p.Provider == models.UpstreamOllama && p.BaseURL == "") || (p.Provider == models.UpstreamCopilot && opts.Provider != "copilot") {
			return "", fmt.Errorf("models.yaml: passthrough.%s: provider %s is not available on this cluster install", p.Name, p.Provider)
		}
	}
	if err := res.ApplyPinball(pinball); err != nil {
		return "", err
	}
	data := newTemplateData(opts, res)
	if opts.StandinPatternsFile != "" {
		raw, err := os.ReadFile(opts.StandinPatternsFile)
		if err != nil {
			return "", fmt.Errorf("reading stand-in patterns file: %w", err)
		}
		data.StandinPatterns = "    " + strings.ReplaceAll(strings.TrimRight(string(raw), "\n"), "\n", "\n    ")
	}

	tmpl, err := template.New("k8s").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("parsing k8s template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering k8s template: %w", err)
	}
	return buf.String(), nil
}

// DeleteNamespace removes the namespace (and everything in it) installed by RenderManifests.
func DeleteNamespace(ns string) error {
	out, err := exec.Command("kubectl", "delete", "namespace", ns, "--ignore-not-found=true").CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete namespace failed: %w (output: %s)", err, string(out))
	}
	return nil
}

func ApplyManifests(manifests string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %w (output: %s)", err, string(out))
	}
	return nil
}

func DeleteManifests(manifests string) error {
	cmd := exec.Command("kubectl", "delete", "-f", "-")
	cmd.Stdin = bytes.NewBufferString(manifests)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete failed: %w (output: %s)", err, string(out))
	}
	return nil
}

// ExtProcTimeoutValue is the effective ext_proc requestTimeout for templates.
func (o *ClusterOptions) ExtProcTimeoutValue() string {
	return config.ExtProcTimeoutOrDefault(o.ExtProcTimeout)
}

// cpuQuantity matches Kubernetes CPU quantities: whole/decimal cores or millicores.
var cpuQuantity = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?|[0-9]+m)$`)

// StandinCPURequestValue is Stand-In's effective CPU request for templates.
func (o *ClusterOptions) StandinCPURequestValue() string {
	return orDefault(o.StandinCPURequest, "50m")
}

// StandinCPULimitValue is Stand-In's effective CPU limit for templates.
func (o *ClusterOptions) StandinCPULimitValue() string { return orDefault(o.StandinCPULimit, "500m") }

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
