package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/telemetry"
)

var (
	statsJSON     bool
	statsWatch    bool
	statsInterval int
	statsHost     string
	statsPorts    [3]int
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show request, redaction and routing stats scraped from Prometheus metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		ep := telemetry.NewEndpoints(statsHost, statsPorts[0], statsPorts[1], statsPorts[2])
		out := cmd.OutOrStdout()
		render := func() *telemetry.Stats {
			st := telemetry.Collect(cmd.Context(), ep)
			if statsJSON {
				b, _ := json.MarshalIndent(st, "", "  ")
				fmt.Fprintln(out, string(b))
			} else {
				telemetry.Print(st, out)
			}
			return st
		}
		if !statsWatch {
			if st := render(); st.AllDown() {
				return fmt.Errorf("all metrics sources unreachable; is the stack running? (convergence start)")
			}
			return nil
		}
		ctx, stop := context.WithCancel(cmd.Context())
		defer stop()
		t := time.NewTicker(time.Duration(statsInterval) * time.Second)
		defer t.Stop()
		for {
			if !statsJSON {
				fmt.Fprint(os.Stdout, "\033[H\033[2J")
			}
			render()
			select {
			case <-ctx.Done():
				return nil
			case <-t.C:
			}
		}
	},
}

func init() {
	f := statsCmd.Flags()
	f.BoolVar(&statsJSON, "json", false, "output JSON")
	f.BoolVar(&statsWatch, "watch", false, "refresh continuously")
	f.IntVar(&statsInterval, "interval", 2, "refresh interval in seconds for --watch")
	f.StringVar(&statsHost, "host", "127.0.0.1", "metrics host")
	f.IntVar(&statsPorts[0], "gateway-metrics-port", 15020, "AgentGateway stats port")
	f.IntVar(&statsPorts[1], "standin-metrics-port", 9102, "Stand-In metrics port")
	f.IntVar(&statsPorts[2], "pinball-metrics-port", 9201, "Pinball metrics port")
	rootCmd.AddCommand(statsCmd)
}
