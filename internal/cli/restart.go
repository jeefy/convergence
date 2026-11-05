package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Re-render configuration and restart the Convergence containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInit(); err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}
		fmt.Printf("Stopping Convergence services with %s...\n", s.eng.Name())
		// down + up (not `compose restart`) so config/.env changes take effect.
		if err := s.eng.Stop(cmd.Context(), s.dir); err != nil {
			return err
		}
		return s.up(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
