package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/engine"
	"github.com/jeefy/convergence/internal/models"
	"github.com/jeefy/convergence/internal/templates"
)

// healthTimeout is a var so tests can shorten it.
var healthTimeout = 90 * time.Second

// stack bundles what every lifecycle command needs.
type stack struct {
	cfg *config.Config
	dir string
	eng *engine.ComposeEngine
}

func loadStack() (*stack, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	dir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}
	eng, err := engine.DetectEngine(cfg.Engine)
	if err != nil {
		return nil, err
	}
	return &stack{cfg: cfg, dir: dir, eng: eng}, nil
}

// requireInit fails when no config exists, instead of starting with defaults.
func requireInit() error {
	ok, err := config.Exists()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("convergence is not initialized; run 'convergence init' first")
	}
	return nil
}

// checkSecrets catches a missing provider credential before compose does.
func checkSecrets(cfg *config.Config) error {
	switch cfg.Provider {
	case config.ProviderCopilot:
		if cfg.CopilotToken == "" {
			return errors.New("copilot provider selected but no GitHub token is configured; run 'convergence login copilot' (or 'convergence init --copilot-token ...')")
		}
	case config.ProviderDirect:
		if cfg.OpenAIKey == "" && cfg.AnthropicKey == "" {
			fmt.Fprintln(os.Stderr, "WARNING: no OpenAI or Anthropic key configured; upstream requests will fail until you run 'convergence init'.")
		}
	case config.ProviderOllama:
	default:
		return fmt.Errorf("unknown provider %q in config", cfg.Provider)
	}
	return nil
}

// up renders assets, brings the stack up and waits for the gateway.
func (s *stack) up(ctx context.Context) error {
	if err := checkSecrets(s.cfg); err != nil {
		return err
	}
	if err := templates.RenderAssets(s.cfg, s.dir); err != nil {
		return fmt.Errorf("rendering assets: %w", err)
	}
	running, err := s.eng.RunningServices(ctx, s.dir)
	if err != nil {
		return err
	}
	// Our own running containers hold these ports; only probe when idle.
	if len(running) == 0 {
		if err := engine.CheckPortsFree(s.cfg.BindAddress, s.cfg.PublishedPorts()); err != nil {
			return err
		}
	}
	if s.cfg.Provider == config.ProviderOllama {
		if err := s.checkOllama(ctx); err != nil {
			return err
		}
	}
	fmt.Printf("Starting Convergence services with %s...\n", s.eng.Name())
	if err := s.eng.Start(ctx, s.dir); err != nil {
		return err
	}
	return s.waitHealthy(ctx)
}

// checkOllama warns (or fails with --strict) about mapped models Ollama lacks.
func (s *stack) checkOllama(ctx context.Context) error {
	res, err := models.ForConfig(s.cfg, s.dir)
	if err != nil {
		return err
	}
	var wanted []string
	for _, m := range res.Models {
		wanted = append(wanted, m.UpstreamModel)
	}
	// The gateway container reaches the host as host.docker.internal; from
	// here that name usually does not resolve, so probe loopback instead.
	url := strings.Replace(s.cfg.OllamaURL, "host.docker.internal", "127.0.0.1", 1)
	missing, err := engine.OllamaMissing(ctx, url, wanted)
	if err != nil {
		msg := fmt.Sprintf("cannot check Ollama models: %v (is Ollama running?)", err)
		if startStrict {
			return errors.New(msg)
		}
		fmt.Fprintln(os.Stderr, "WARNING: "+msg)
		return nil
	}
	if len(missing) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("Ollama is missing mapped models; pull them with:\n")
	for _, m := range missing {
		fmt.Fprintf(&b, "  ollama pull %s\n", m)
	}
	if startStrict {
		return errors.New(strings.TrimSpace(b.String()))
	}
	fmt.Fprint(os.Stderr, "WARNING: "+b.String())
	return nil
}

func (s *stack) waitHealthy(ctx context.Context) error {
	fmt.Println("Waiting for AgentGateway to become healthy...")
	hctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	if err := s.eng.WaitHealthy(hctx, s.cfg.HealthHost(), s.cfg.GatewayPort); err != nil {
		return fmt.Errorf("%w (inspect with 'convergence logs agentgateway')", err)
	}
	fmt.Println("AgentGateway is healthy and ready!")
	fmt.Printf("\nBase URL: http://%s:%d/v1\n", s.cfg.HealthHost(), s.cfg.GatewayPort)
	fmt.Println("API Key:  convergence (or any non-empty string)")
	return nil
}

var startStrict bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Convergence AI Gateway containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInit(); err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}
		return s.up(cmd.Context())
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Convergence AI Gateway containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadStack()
		if err != nil {
			return err
		}
		fmt.Printf("Stopping Convergence services with %s...\n", s.eng.Name())
		return s.eng.Stop(cmd.Context(), s.dir)
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop and remove Convergence containers, networks, and volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadStack()
		if err != nil {
			return err
		}
		fmt.Printf("Tearing down Convergence services with %s...\n", s.eng.Name())
		return s.eng.Down(cmd.Context(), s.dir)
	},
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune dangling container images, stopped containers, and unused networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadStack()
		if err != nil {
			return err
		}
		fmt.Printf("Pruning dangling containers and images with %s...\n", s.eng.Name())
		return s.eng.Prune(cmd.Context(), s.dir)
	},
}

func init() {
	startCmd.Flags().BoolVar(&startStrict, "strict", false, "Fail (instead of warn) when Ollama lacks a mapped model or is unreachable")
	rootCmd.AddCommand(startCmd, stopCmd, downCmd, pruneCmd)
}
