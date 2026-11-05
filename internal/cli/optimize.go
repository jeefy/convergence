package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Run or schedule Pinball model re-ranking and usage evaluation",
	Long: `Executes the Pinball optimizer to analyze recent gateway usage,
replay sampled requests against candidate models in their tier, and
evaluate model assignments and within-tier rankings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		checkRevert, _ := cmd.Flags().GetBool("check-revert")

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		cfgDir, err := config.GetConfigDir()
		if err != nil {
			return err
		}

		fmt.Println("Running Pinball optimizer evaluation...")

		dockerArgs := []string{
			"run", "--rm",
			"-v", fmt.Sprintf("%s:/etc/pinball:ro", cfgDir),
			"-e", "PINBALL_CONFIG=/etc/pinball/pinball.yaml",
			"-e", fmt.Sprintf("PINBALL_OPT_GATEWAY_URL=http://host.docker.internal:%d", cfg.GatewayPort),
			"-e", "PINBALL_OPT_MODEL_PREFIX=opt-",
			"-e", "PINBALL_OPT_BUDGET_USD=10.0",
		}

		// Mount log directory if exists for local inspection
		logDir := filepath.Join(cfgDir, "logs")
		if _, err := os.Stat(logDir); err == nil {
			dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/var/log/agentgateway:ro", logDir))
		}

		dockerArgs = append(dockerArgs, "ghcr.io/jeefy/pinball:0.7.2", "pinball-optimize")

		if checkRevert {
			dockerArgs = append(dockerArgs, "--check-revert")
		} else if dryRun {
			dockerArgs = append(dockerArgs, "--dry-run", "run")
		} else {
			dockerArgs = append(dockerArgs, "run")
		}

		engineCmd := exec.Command(cfg.Engine, dockerArgs...)
		engineCmd.Stdout = os.Stdout
		engineCmd.Stderr = os.Stderr

		if err := engineCmd.Run(); err != nil {
			return fmt.Errorf("pinball optimizer run failed: %w", err)
		}

		fmt.Println("Pinball optimization evaluation completed.")
		return nil
	},
}

func init() {
	optimizeCmd.Flags().Bool("dry-run", false, "Sample and plan without executing replay model calls")
	optimizeCmd.Flags().Bool("check-revert", false, "Check if recent model re-ranking regressed and revert if needed")
	rootCmd.AddCommand(optimizeCmd)
}
