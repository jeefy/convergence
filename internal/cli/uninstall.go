package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/templates"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Convergence containers, networks and volumes (--purge also deletes ~/.convergence)",
	RunE: func(cmd *cobra.Command, args []string) error {
		purge, err := cmd.Flags().GetBool("purge")
		if err != nil {
			return err
		}
		yes, err := cmd.Flags().GetBool("yes")
		if err != nil {
			return err
		}
		s, err := loadStack()
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(s.dir, templates.ComposeFile)); err == nil {
			fmt.Printf("Removing Convergence containers and volumes with %s...\n", s.eng.Name())
			if err := s.eng.Down(cmd.Context(), s.dir); err != nil {
				return err
			}
		} else {
			fmt.Println("No rendered compose file found; nothing to tear down.")
		}
		if !purge {
			fmt.Printf("Containers removed. Configuration kept in %s (use --purge to delete it).\n", s.dir)
			return nil
		}
		if !yes {
			ok, err := confirm(bufio.NewReader(os.Stdin), fmt.Sprintf("Delete %s including API keys and the Stand-In salt?", s.dir))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Aborted; configuration kept.")
				return nil
			}
		}
		if err := os.RemoveAll(s.dir); err != nil {
			return fmt.Errorf("removing %s: %w", s.dir, err)
		}
		fmt.Printf("Removed %s.\n", s.dir)
		return nil
	},
}

func init() {
	uninstallCmd.Flags().Bool("purge", false, "Also delete ~/.convergence (config, keys, salt)")
	uninstallCmd.Flags().BoolP("yes", "y", false, "Do not ask for confirmation")
	rootCmd.AddCommand(uninstallCmd)
}
