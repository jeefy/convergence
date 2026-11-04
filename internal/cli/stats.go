package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/telemetry"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display AI Gateway token usage, cost, and routing stats",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}

		logDir := filepath.Join(dir, "logs")
		summary, err := telemetry.ParseAccessLogs(logDir)
		if err != nil {
			return err
		}

		telemetry.PrintStats(summary, os.Stdout)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
