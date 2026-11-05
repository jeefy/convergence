package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/templates"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Re-render assets with this binary's pinned images, pull them and recreate the stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireInit(); err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}
		if err := templates.RenderAssets(s.cfg, s.dir); err != nil {
			return fmt.Errorf("rendering assets: %w", err)
		}
		fmt.Printf("Pulling pinned images with %s...\n", s.eng.Name())
		if err := s.eng.Pull(cmd.Context(), s.dir); err != nil {
			return err
		}
		// `up -d` recreates only containers whose image or config changed.
		return s.up(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
