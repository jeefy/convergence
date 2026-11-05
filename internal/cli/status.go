package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/health"
)

var statusJSON bool

type statusReport struct {
	Engine   string                 `json:"engine"`
	Provider string                 `json:"provider"`
	Endpoint string                 `json:"endpoint"`
	FailOpen bool                   `json:"fail_open"`
	OK       bool                   `json:"ok"`
	Services []health.ServiceStatus `json:"services"`
}

func writeStatus(w io.Writer, r statusReport, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	_, _ = fmt.Fprintln(w, "=== Convergence Status ===")
	_, _ = fmt.Fprintf(w, "Container Engine: %s\n", r.Engine)
	_, _ = fmt.Fprintf(w, "Active Provider:  %s\n", r.Provider)
	_, _ = fmt.Fprintf(w, "Gateway Endpoint: %s\n", r.Endpoint)
	if r.FailOpen {
		_, _ = fmt.Fprintln(w, "Stand-In mode:    fail-open (UNSCRUBBED passthrough if down)")
	} else {
		_, _ = fmt.Fprintln(w, "Stand-In mode:    fail-closed")
	}
	_, _ = fmt.Fprintln(w)
	for _, s := range r.Services {
		state := "OK"
		if !s.Running || !s.Healthy {
			state = "DOWN"
		}
		_, _ = fmt.Fprintf(w, "%-13s %-4s container=%-11s healthy=%-5t %s\n", s.Name, state, s.Container, s.Healthy, s.Detail)
	}
	if b := health.StandInBanner(r.Services, r.FailOpen); b != "" {
		_, _ = fmt.Fprintf(w, "\n%s", b)
	}
	return nil
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report each Convergence service (container state + health probe); exits non-zero if any is down",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadStack()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		ss := health.CheckWith(ctx, s.cfg, s.eng, s.dir)
		r := statusReport{
			Engine:   s.eng.Name(),
			Provider: string(s.cfg.Provider),
			Endpoint: fmt.Sprintf("http://%s:%d/v1", s.cfg.HealthHost(), s.cfg.GatewayPort),
			FailOpen: s.cfg.FailOpen,
			OK:       health.AllRequiredUp(ss),
			Services: ss,
		}
		if err := writeStatus(cmd.OutOrStdout(), r, statusJSON); err != nil {
			return err
		}
		if !r.OK {
			cmd.SilenceUsage, cmd.SilenceErrors = true, statusJSON
			return errors.New("one or more required services are down")
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Machine-readable output")
	rootCmd.AddCommand(statusCmd)
}
