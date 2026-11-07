package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/engine"
	"github.com/jeefy/convergence/internal/templates"
)

// initFlags are the non-interactive overrides for `convergence init`.
// Empty/zero values mean "not given": fall back to a prompt (or keep existing).
type initFlags struct {
	yes          bool
	provider     string
	engine       string
	sandbox      string
	copilotToken string
	openAIKey    string
	anthropicKey string
	ollamaURL    string
	port         int
	bindAddress  string
	failOpen     bool
	requestLogDB string
	otel         config.OTel
	ollamaLight  string
	ollamaStd    string
	ollamaHeavy  string

	imageAGW        string
	imageStandIn    string
	imagePinball    string
	imageCopilotAPI string
	imageOpenShell  string
}

var initOpts initFlags

// prompter asks questions on in/out; with yes=true it never reads input.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
	yes bool
}

// ask prints question and returns the trimmed answer ("" on EOF).
func (p *prompter) ask(question string) (string, error) {
	_, _ = fmt.Fprint(p.out, question)
	line, err := p.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func confirm(in *bufio.Reader, question string) (bool, error) {
	p := prompter{in: in, out: os.Stdout}
	ans, err := p.ask(question + " [y/N]: ")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes"), nil
}

// secretField resolves one credential: flag > prompt > existing value.
// A flag that would replace a different existing value needs confirmation.
func (p *prompter) secretField(label, flagVal, existing string) (string, error) {
	if flagVal != "" {
		if existing == "" || existing == flagVal || p.yes {
			return flagVal, nil
		}
		ok, err := confirm(p.in, fmt.Sprintf("Replace the existing %s?", label))
		if err != nil {
			return existing, err
		}
		if !ok {
			return existing, fmt.Errorf("refusing to overwrite existing %s (re-run with --yes to replace it)", label)
		}
		return flagVal, nil
	}
	if p.yes {
		return existing, nil
	}
	hint := "or press Enter to skip"
	if existing != "" {
		hint = "or press Enter to keep the existing value"
	}
	ans, err := p.ask(fmt.Sprintf("Enter %s (%s): ", label, hint))
	if err != nil {
		return existing, err
	}
	if ans == "" {
		return existing, nil
	}
	return ans, nil
}

func (p *prompter) chooseProvider(flagVal string, current config.ProviderType) (config.ProviderType, error) {
	if flagVal != "" {
		return config.ParseProvider(flagVal)
	}
	if p.yes {
		return current, nil
	}
	choices := []config.ProviderType{config.ProviderCopilot, config.ProviderDirect, config.ProviderOllama}
	def := "1"
	for i, c := range choices {
		if c == current {
			def = fmt.Sprint(i + 1)
		}
	}
	_, _ = fmt.Fprintln(p.out, "Select your primary LLM backend:")
	_, _ = fmt.Fprintln(p.out, "  [1] GitHub Copilot (Subscription token)")
	_, _ = fmt.Fprintln(p.out, "  [2] Direct API Keys (OpenAI / Anthropic)")
	_, _ = fmt.Fprintln(p.out, "  [3] Local Only (Ollama)")
	ans, err := p.ask(fmt.Sprintf("Enter choice [1-3] (default %s): ", def))
	if err != nil {
		return "", err
	}
	if ans == "" {
		ans = def
	}
	switch ans {
	case "1", "2", "3":
		return choices[ans[0]-'1'], nil
	default:
		return "", fmt.Errorf("invalid choice %q", ans)
	}
}

// resolveConfig merges flags and prompt answers into cfg (loaded or default).
// The Stand-In salt is never touched here.
func resolveConfig(cfg *config.Config, f initFlags, p *prompter) error {
	provider, err := p.chooseProvider(f.provider, cfg.Provider)
	if err != nil {
		return err
	}
	cfg.Provider = provider
	if f.port != 0 {
		if f.port < 1 || f.port > 65535 {
			return fmt.Errorf("invalid --port %d", f.port)
		}
		cfg.GatewayPort = f.port
	}
	if f.bindAddress != "" {
		cfg.BindAddress = f.bindAddress
	}
	if f.failOpen {
		cfg.FailOpen = true
	}
	if f.requestLogDB != "" {
		cfg.RequestLogDB = f.requestLogDB
	}
	if f.otel.Endpoint != "" {
		cfg.OTel.Endpoint = f.otel.Endpoint
	}
	if f.otel.Protocol != "" {
		cfg.OTel.Protocol = f.otel.Protocol
	}
	if f.otel.Sample != 0 {
		cfg.OTel.Sample = f.otel.Sample
	}
	if err := cfg.OTel.Validate(); err != nil {
		return err
	}
	if f.ollamaLight != "" {
		cfg.OllamaModelLight = f.ollamaLight
	}
	if f.ollamaStd != "" {
		cfg.OllamaModelStandard = f.ollamaStd
	}
	if f.ollamaHeavy != "" {
		cfg.OllamaModelHeavy = f.ollamaHeavy
	}

	switch provider {
	case config.ProviderCopilot:
		if cfg.CopilotToken, err = p.secretField("GitHub Copilot token", f.copilotToken, cfg.CopilotToken); err != nil {
			return err
		}
	case config.ProviderDirect:
		if cfg.AnthropicKey, err = p.secretField("Anthropic API key", f.anthropicKey, cfg.AnthropicKey); err != nil {
			return err
		}
		if cfg.OpenAIKey, err = p.secretField("OpenAI API key", f.openAIKey, cfg.OpenAIKey); err != nil {
			return err
		}
	case config.ProviderOllama:
		switch {
		case f.ollamaURL != "":
			cfg.OllamaURL = f.ollamaURL
		case !p.yes:
			ans, err := p.ask(fmt.Sprintf("Enter Ollama URL (default %s): ", cfg.OllamaURL))
			if err != nil {
				return err
			}
			if ans != "" {
				cfg.OllamaURL = ans
			}
		}
	}

	if f.sandbox != "" {
		sbx, err := config.ParseSandbox(f.sandbox)
		if err != nil {
			return err
		}
		cfg.Sandbox = sbx
	}

	if f.imageAGW != "" {
		cfg.ImageAgentGateway = f.imageAGW
	}
	if f.imageStandIn != "" {
		cfg.ImageStandIn = f.imageStandIn
	}
	if f.imagePinball != "" {
		cfg.ImagePinball = f.imagePinball
	}
	if f.imageCopilotAPI != "" {
		cfg.ImageCopilotAPI = f.imageCopilotAPI
	}
	if f.imageOpenShell != "" {
		cfg.ImageOpenShellGateway = f.imageOpenShell
	}
	return nil
}

func detectInitEngine(flagVal string) (*engine.ComposeEngine, error) {
	switch flagVal {
	case "":
		return engine.DetectEngine("")
	case "docker", "podman":
		eng, err := engine.DetectEngine(flagVal)
		if err != nil {
			return nil, err
		}
		if eng.Name() != flagVal {
			return nil, fmt.Errorf("--engine %s requested but %s is not on PATH", flagVal, flagVal)
		}
		return eng, nil
	default:
		return nil, fmt.Errorf("invalid --engine %q (want docker or podman)", flagVal)
	}
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up Convergence (interactive, or non-interactive with flags and --yes)",
	Long: `Writes ~/.convergence/config.yaml and renders the compose stack.

Re-running init is safe: the Stand-In salt is preserved, and existing keys are
kept unless you enter a new value or confirm replacing them (--yes skips all
prompts and confirmations).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Convergence Setup Wizard ===")
		eng, err := detectInitEngine(initOpts.engine)
		if err != nil {
			return err
		}
		fmt.Printf("Using container engine: %s\n\n", eng.Name())

		// LoadConfig returns defaults (fresh salt) only when no config exists;
		// a corrupt config is an error, never silently replaced.
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		cfg.Engine = eng.Name()

		p := &prompter{in: bufio.NewReader(os.Stdin), out: os.Stdout, yes: initOpts.yes}
		if err := resolveConfig(cfg, initOpts, p); err != nil {
			return err
		}
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		// Starter models.yaml is written only when absent (never clobbered).
		if err := writeStarterModels(dir); err != nil {
			return err
		}
		if err := templates.RenderAssets(cfg, dir); err != nil {
			return fmt.Errorf("rendering templates: %w", err)
		}

		fmt.Println("\nConfiguration rendered successfully to:", dir)
		if cfg.Provider == config.ProviderCopilot && cfg.CopilotToken == "" {
			fmt.Println("No Copilot token yet: run 'convergence login copilot' before starting.")
		}
		if cfg.FailOpen {
			fmt.Println("WARNING: --fail-open is on: if Stand-In is down, requests go upstream UNSCRUBBED.")
		}
		fmt.Println("Run 'convergence start' to launch your gateway.")
		return nil
	},
}

func init() {
	f := initCmd.Flags()
	f.BoolVarP(&initOpts.yes, "yes", "y", false, "Non-interactive: never prompt, keep existing values, allow overwriting keys")
	f.StringVar(&initOpts.provider, "provider", "", "LLM backend: copilot, direct or ollama")
	f.StringVar(&initOpts.engine, "engine", "", "Container engine: docker or podman (default: auto-detect)")
	f.StringVar(&initOpts.copilotToken, "copilot-token", "", "GitHub OAuth token for copilot-api (see 'convergence login copilot')")
	f.StringVar(&initOpts.openAIKey, "openai-key", "", "OpenAI API key (direct provider)")
	f.StringVar(&initOpts.anthropicKey, "anthropic-key", "", "Anthropic API key (direct provider)")
	f.StringVar(&initOpts.ollamaURL, "ollama-url", "", "Ollama base URL (ollama provider)")
	f.BoolVar(&initOpts.failOpen, "fail-open", false, "If Stand-In is down, let requests continue UNSCRUBBED to the upstream (default: fail closed, reject them)")
	f.StringVar(&initOpts.requestLogDB, "request-log-db", "", "Postgres DSN of a request-log database for agentgateway, used by 'convergence optimize' (default: off; stores prompts and completions)")
	f.StringVar(&initOpts.otel.Endpoint, "otel-endpoint", "", "Export OTLP traces to this collector host:port (default: off)")
	f.StringVar(&initOpts.otel.Protocol, "otel-protocol", "", "OTLP protocol: grpc (default) or http")
	f.Float64Var(&initOpts.otel.Sample, "otel-sample", 0, "Trace sampling rate 0..1 (default 1)")
	f.StringVar(&initOpts.ollamaLight, "ollama-model-light", "", "Ollama model for the light tier (default from 'convergence models')")
	f.StringVar(&initOpts.ollamaStd, "ollama-model-standard", "", "Ollama model for the standard tier")
	f.StringVar(&initOpts.ollamaHeavy, "ollama-model-heavy", "", "Ollama model for the heavy tier")
	f.IntVar(&initOpts.port, "port", 0, "Host port for the gateway (default 3000)")
	f.StringVar(&initOpts.bindAddress, "bind-address", "", "Host address to publish ports on (default 127.0.0.1; 0.0.0.0 exposes to the network)")
	f.StringVar(&initOpts.imageAGW, "image-agentgateway", "", "Override agentgateway image (tag or digest)")
	f.StringVar(&initOpts.imageStandIn, "image-standin", "", "Override stand-in image (tag or digest)")
	f.StringVar(&initOpts.imagePinball, "image-pinball", "", "Override pinball image (tag or digest)")
	f.StringVar(&initOpts.imageCopilotAPI, "image-copilot-api", "", "Override copilot-api image (tag or digest)")
	f.StringVar(&initOpts.imageOpenShell, "image-openshell", "", "Override openshell gateway image (tag or digest)")
	f.StringVar(&initOpts.sandbox, "sandbox", "", "Sandboxing runtime: none, openshell, nono, docker-sbx, or k8s-agent (default: none)")
	rootCmd.AddCommand(initCmd)
}
