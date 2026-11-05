package cli

import (
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:       "logs [service...]",
	Short:     "Show container logs (agentgateway, stand-in, pinball, copilot-api)",
	ValidArgs: []string{"agentgateway", "stand-in", "pinball", "copilot-api"},
	Args:      cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, err := cmd.Flags().GetBool("follow")
		if err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}
		return s.eng.Logs(cmd.Context(), s.dir, follow, args...)
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	rootCmd.AddCommand(logsCmd)
}
