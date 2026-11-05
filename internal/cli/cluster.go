package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/k8s"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes cluster deployments of Convergence",
}

var clusterInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Convergence onto your active Kubernetes cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		prov, _ := cmd.Flags().GetString("provider")
		copilotTok, _ := cmd.Flags().GetString("copilot-token")
		openaiKey, _ := cmd.Flags().GetString("openai-key")
		anthropicKey, _ := cmd.Flags().GetString("anthropic-key")
		optSchedule, _ := cmd.Flags().GetString("opt-schedule")
		optDSN, _ := cmd.Flags().GetString("optimizer-dsn")
		failOpen, _ := cmd.Flags().GetBool("fail-open")
		openaiBase, _ := cmd.Flags().GetString("openai-base-url")
		modelsFile, _ := cmd.Flags().GetString("models-file")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		opts := &k8s.ClusterOptions{
			Namespace:     ns,
			Provider:      prov,
			CopilotToken:  copilotTok,
			OpenAIKey:     openaiKey,
			AnthropicKey:  anthropicKey,
			OptSchedule:   optSchedule,
			OptimizerDSN:  optDSN,
			FailOpen:      failOpen,
			OpenAIBaseURL: openaiBase,
			ModelsFile:    modelsFile,
			DryRun:        dryRun,
		}

		manifests, err := k8s.RenderManifests(opts)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Println(manifests)
			return nil
		}

		fmt.Printf("Applying Convergence manifests to namespace '%s'...\n", ns)
		if err := k8s.ApplyManifests(manifests); err != nil {
			return err
		}

		fmt.Printf("Successfully deployed Convergence to cluster in namespace '%s'!\n", ns)
		return nil
	},
}

var clusterUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Convergence (the whole namespace) from the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Printf("This deletes namespace '%s' and everything in it. Re-run with --yes to confirm.\n", ns)
			return fmt.Errorf("confirmation required")
		}
		if err := k8s.DeleteNamespace(ns); err != nil {
			return err
		}
		fmt.Printf("Deleted namespace '%s'.\n", ns)
		return nil
	},
}

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of cluster deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		fmt.Printf("Checking pods in namespace '%s'...\n", ns)
		c := exec.Command("kubectl", "-n", ns, "get", "pods", "-o", "wide")
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("kubectl get pods failed: %w (output: %s)", err, string(out))
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	clusterInstallCmd.Flags().String("namespace", "convergence", "Target Kubernetes namespace")
	clusterInstallCmd.Flags().String("provider", "copilot", "LLM backend provider: copilot or direct")
	clusterInstallCmd.Flags().String("copilot-token", "", "GitHub Copilot token")
	clusterInstallCmd.Flags().String("openai-key", "", "OpenAI API key")
	clusterInstallCmd.Flags().String("anthropic-key", "", "Anthropic API key")
	clusterInstallCmd.Flags().String("opt-schedule", "0 3 * * 0", "Cron schedule for pinball optimizer evaluation checks (default: weekly Sunday 03:00)")
	clusterInstallCmd.Flags().String("optimizer-dsn", "", "Read-only Postgres DSN for the gateway request_logs; enables the pinball optimizer CronJob (stored in Secret pinball-optimizer-db)")
	clusterInstallCmd.Flags().Bool("fail-open", false, "If Stand-In is down, let requests through anyway: they then go UNSCRUBBED to the upstream (default: fail closed, reject them)")
	clusterInstallCmd.Flags().String("openai-base-url", "", "Base URL of an OpenAI-compatible endpoint for provider=direct (testing / self-hosted)")
	_ = clusterInstallCmd.Flags().MarkHidden("openai-base-url")
	clusterInstallCmd.Flags().String("models-file", "", "models.yaml override file (same schema as ~/.convergence/models.yaml) for the tier->upstream model mapping")
	clusterInstallCmd.Flags().Bool("dry-run", false, "Output rendered YAML to stdout without applying")

	clusterStatusCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")

	clusterUninstallCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")
	clusterUninstallCmd.Flags().Bool("yes", false, "Confirm deletion")

	clusterCmd.AddCommand(clusterInstallCmd)
	clusterCmd.AddCommand(clusterUninstallCmd)
	clusterCmd.AddCommand(clusterStatusCmd)
	rootCmd.AddCommand(clusterCmd)
}
