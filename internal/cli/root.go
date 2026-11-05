package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/version"
)

var rootCmd = &cobra.Command{
	Use:   "convergence",
	Short: "Convergence: Turnkey AI Gateway, Secret Stripping & Pinball Autopilot",
	Long: `Convergence orchestrates AgentGateway, Stand-In secret tokenization,
and Pinball autopilot routing into a unified, zero-friction AI gateway.`,
	// Runtime errors are not usage errors: print the error, not the help text.
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("convergence version %s (%s, built %s)\n", version.Version, version.Commit, version.Date)
	},
}
