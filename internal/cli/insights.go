package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/insights"
)

var (
	insightsSince      string
	insightsCoach      bool
	insightsJSON       bool
	insightsDBDSN      string
	insightsGatewayURL string
)

var insightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Analyze prompt and response activity for usage patterns and coaching",
	Long: `Analyze recent prompts, completions, token usage, and tier routing recorded
in agentgateway's Postgres request log over 1d, 7d, or 30d.

Provides deterministic analysis (token volumes, tier skew, context bloat, retry loops),
and optional AI coaching advice (--coach) synthesized through the local Convergence gateway.

Requires a request-log database configured with full LLM logging (init --request-log-db
or cluster install --request-log-postgres / --request-log-db), or pass --db-dsn.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, err := parseSince(insightsSince)
		if err != nil {
			return err
		}

		dsn := insightsDBDSN
		gatewayURL := insightsGatewayURL

		// Attempt to load default DSN and gateway URL from stack config if not passed via flag
		if dsn == "" || gatewayURL == "" {
			if s, err := loadStack(); err == nil && s != nil {
				if dsn == "" {
					dsn = s.cfg.RequestLogDB
				}
				if gatewayURL == "" {
					gatewayURL = fmt.Sprintf("http://127.0.0.1:%d", s.cfg.GatewayPort)
				}
			}
		}

		if dsn == "" {
			return errors.New("insights requires agentgateway's Postgres request log: run 'convergence init --request-log-db <postgres-dsn>' or pass --db-dsn")
		}

		ctx := cmd.Context()
		records, err := insights.FetchRequests(ctx, dsn, duration)
		if err != nil {
			return fmt.Errorf("fetching request logs: %w", err)
		}

		assessment := insights.Analyze(records, insightsSince)

		var coachAdvice string
		if insightsCoach {
			if len(records) == 0 {
				coachAdvice = "No activity in this window to analyze."
			} else {
				advice, err := insights.GenerateCoachAdvice(ctx, gatewayURL, &assessment)
				if err != nil {
					coachAdvice = fmt.Sprintf("[Coaching generation unavailable: %v]", err)
				} else {
					coachAdvice = advice
				}
			}
		}

		out := cmd.OutOrStdout()
		if insightsJSON {
			return insights.PrintJSON(out, &assessment, coachAdvice)
		}
		return insights.PrintAssessment(out, &assessment, coachAdvice)
	},
}

func parseSince(s string) (time.Duration, error) {
	switch s {
	case "1d":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	case "30d":
		return 30 * 24 * time.Hour, nil
	default:
		// Also accept standard Go durations like 12h, 48h
		d, err := time.ParseDuration(s)
		if err == nil && d > 0 {
			return d, nil
		}
		return 0, fmt.Errorf("invalid --since %q: expected 1d, 7d, 30d, or a valid duration (e.g. 24h)", s)
	}
}

func init() {
	insightsCmd.Flags().StringVar(&insightsSince, "since", "7d", "Time window to analyze (1d, 7d, 30d)")
	insightsCmd.Flags().BoolVar(&insightsCoach, "coach", false, "Generate AI-assisted coaching advice via the Convergence gateway")
	insightsCmd.Flags().BoolVar(&insightsJSON, "json", false, "Output insights in JSON format")
	insightsCmd.Flags().StringVar(&insightsDBDSN, "db-dsn", "", "Postgres DSN of the request-log database (falls back to config request_log_db)")
	insightsCmd.Flags().StringVar(&insightsGatewayURL, "gateway-url", "", "Convergence gateway URL for coaching generation (default: http://127.0.0.1:<port>)")
	rootCmd.AddCommand(insightsCmd)
}
