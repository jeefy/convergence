package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/models"
)

// writeStarterModels creates dir/models.yaml only when it does not exist.
func writeStarterModels(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, models.FileName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("writing starter models.yaml: %w", err)
	}
	defer func() { _ = f.Close() }() // write error is returned below; close is best-effort
	_, err = f.WriteString(models.StarterFile())
	return err
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Show the active pinball tier -> upstream model mapping",
	Long: `Prints which upstream model serves each pinball tier for the configured
provider. Defaults come from the binary; override per provider in
~/.convergence/models.yaml (see 'convergence models init').`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		res, err := models.ForConfig(cfg, dir)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provider: %s\n", res.Provider)
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "TIER\tPINBALL MODEL\tUPSTREAM")
		for _, m := range res.Models {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s/%s\n", m.Tier, m.PinballName, m.UpstreamProvider, m.UpstreamModel)
		}
		return w.Flush()
	},
}

var modelsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a commented starter ~/.convergence/models.yaml (never overwrites)",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		path := filepath.Join(dir, models.FileName)
		if _, err := os.Stat(path); err == nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s already exists; leaving it untouched\n", path)
			return nil
		}
		if err := writeStarterModels(dir); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", path)
		return nil
	},
}

func init() {
	modelsCmd.AddCommand(modelsInitCmd)
	rootCmd.AddCommand(modelsCmd)
}
