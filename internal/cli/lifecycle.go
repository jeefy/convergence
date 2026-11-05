package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/engine"
	"github.com/jeefy/convergence/internal/templates"
)

const healthTimeout = 90 * time.Second

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
	fmt.Printf("Starting Convergence services with %s...\n", s.eng.Name())
	if err := s.eng.Start(ctx, s.dir); err != nil {
		return err
	}
	return s.waitHealthy(ctx)
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of Convergence services",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadStack()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		fmt.Println("=== Convergence Status ===")
		fmt.Printf("Container Engine: %s\n", s.eng.Name())
		fmt.Printf("Active Provider:  %s\n", s.cfg.Provider)
		fmt.Printf("Gateway Endpoint: http://%s:%d/v1\n", s.cfg.HealthHost(), s.cfg.GatewayPort)
		if running, err := s.eng.RunningServices(ctx, s.dir); err != nil {
			fmt.Printf("Services:         unknown (%v)\n", err)
		} else {
			fmt.Printf("Services:         %d running %v\n", len(running), running)
		}
		if err := s.eng.CheckHealth(ctx, s.cfg.HealthHost(), s.cfg.GatewayPort); err != nil {
			fmt.Printf("Health:           DOWN (%v)\n", err)
		} else {
			fmt.Printf("Health:           ONLINE (200 OK)\n")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd, stopCmd, statusCmd)
}
