package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/doctor"
	"github.com/jeefy/convergence/internal/health"
)

var (
	doctorJSON    bool
	doctorE2E     bool
	doctorTimeout time.Duration
	doctorGateway string
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the local stack end to end",
	Long: `Runs self-checks (config, engine, services, ports, Stand-In, Pinball, provider,
fail mode). Exits 1 if any check FAILs.

--e2e additionally sends ONE tiny real request (model "auto", max_tokens 16)
containing a fake GitHub token through the gateway. With a real provider this
may cost a few tokens. It is off by default.`,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), doctorTimeout)
		defer cancel()
		results := runDoctor(ctx)
		out := cmd.OutOrStdout()
		if doctorJSON {
			if err := doctor.WriteJSON(out, results); err != nil {
				return err
			}
		} else {
			doctor.WriteText(out, results)
		}
		if doctor.ExitCode(results) != 0 {
			cmd.SilenceUsage = true
			return errors.New("doctor found failing checks")
		}
		return nil
	},
}

func runDoctor(ctx context.Context) []doctor.Result {
	dir, err := config.GetConfigDir()
	if err != nil {
		return []doctor.Result{{Name: "config", Status: doctor.Fail, Detail: err.Error()}}
	}
	cfgRes, cfg := doctor.CheckConfig(dir)
	if cfg == nil {
		return []doctor.Result{cfgRes}
	}
	env := doctor.NewEnv(cfg, dir)
	env.FailOpen = doctor.ReadFailOpen(dir)
	if doctorGateway != "" {
		env.GatewayURL = doctorGateway
	}
	env.Run = func(cmd *exec.Cmd) error {
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %.120s", err, out)
		}
		return nil
	}
	env.Services = newServiceChecker(cfg, dir)
	return append([]doctor.Result{cfgRes}, doctor.Run(ctx, env, doctor.Options{E2E: doctorE2E})...)
}

func init() {
	f := doctorCmd.Flags()
	f.BoolVar(&doctorJSON, "json", false, "machine-readable output")
	f.BoolVar(&doctorE2E, "e2e", false, "send one real request through the gateway (may cost a few tokens)")
	f.DurationVar(&doctorTimeout, "timeout", 60*time.Second, "overall timeout")
	f.StringVar(&doctorGateway, "gateway-url", "", "override gateway base URL (default http://<bind>:<port>)")
	rootCmd.AddCommand(doctorCmd)
}

// healthServices adapts the health package to doctor.ServiceChecker.
type healthServices struct{ cfg *config.Config }

func newServiceChecker(cfg *config.Config, _ string) doctor.ServiceChecker {
	return healthServices{cfg}
}

func (h healthServices) Check(ctx context.Context) []doctor.ServiceStatus {
	sts := health.Check(ctx, h.cfg)
	out := make([]doctor.ServiceStatus, len(sts))
	for i, s := range sts {
		out[i] = doctor.ServiceStatus{Name: s.Name, Container: s.Container, Running: s.Running, Healthy: s.Healthy, Detail: s.Detail}
	}
	return out
}
