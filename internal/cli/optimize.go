package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/templates"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "[EXPERIMENTAL] Run Pinball model re-ranking against the gateway request log",
	Long: `EXPERIMENTAL. Runs pinball-optimize once in a container: it samples recent
requests from agentgateway's Postgres request log, replays them against candidate
models in their tier through the gateway, and re-ranks models within tiers.

The local compose stack does not run Postgres or log requests to a database, so
this command requires --db-dsn pointing at an agentgateway request-log database
reachable from a container (use host.docker.internal for services on this host).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, err := cmd.Flags().GetBool("dry-run")
		if err != nil {
			return err
		}
		checkRevert, err := cmd.Flags().GetBool("check-revert")
		if err != nil {
			return err
		}
		dsn, err := cmd.Flags().GetString("db-dsn")
		if err != nil {
			return err
		}
		if dsn == "" {
			return errors.New("optimize is EXPERIMENTAL and needs agentgateway's Postgres request log, which the local stack does not provide; pass --db-dsn postgres://user:pass@host:5432/agentgateway")
		}
		if err := requireInit(); err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}

		runArgs := []string{
			"run", "--rm",
			// host-gateway works for docker and podman (incl. rootless) on Linux.
			"--add-host=host.docker.internal:host-gateway",
			"-v", fmt.Sprintf("%s/pinball.yaml:/etc/pinball/pinball.yaml:ro%s", s.dir, templates.VolumeSuffix()),
			"-e", "PINBALL_CONFIG=/etc/pinball/pinball.yaml",
			"-e", fmt.Sprintf("PINBALL_OPT_GATEWAY_URL=http://host.docker.internal:%d", s.cfg.GatewayPort),
			"-e", "PINBALL_OPT_MODEL_PREFIX=opt-",
			"-e", "PINBALL_OPT_BUDGET_USD=10.0",
			// Passed by name so the DSN is not visible in the process list.
			"-e", "PINBALL_OPT_DB_DSN",
			templates.ResolveImages(s.cfg).Pinball, "pinball-optimize",
		}
		switch {
		case checkRevert:
			runArgs = append(runArgs, "--check-revert")
		case dryRun:
			runArgs = append(runArgs, "--dry-run", "run")
		default:
			runArgs = append(runArgs, "run")
		}

		fmt.Println("Running Pinball optimizer (EXPERIMENTAL)...")
		c := exec.CommandContext(cmd.Context(), s.eng.Name(), runArgs...)
		c.Env = append(os.Environ(), "PINBALL_OPT_DB_DSN="+dsn)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("pinball optimizer run failed: %w", err)
		}
		fmt.Println("Pinball optimization evaluation completed.")
		return nil
	},
}

func init() {
	optimizeCmd.Flags().Bool("dry-run", false, "Sample and plan without executing replay model calls")
	optimizeCmd.Flags().Bool("check-revert", false, "Check if recent model re-ranking regressed and revert if needed")
	optimizeCmd.Flags().String("db-dsn", "", "Postgres DSN of the agentgateway request log (required)")
	rootCmd.AddCommand(optimizeCmd)
}
