package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
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
		rlDB, _ := cmd.Flags().GetString("request-log-db")
		rlPG, _ := cmd.Flags().GetBool("request-log-postgres")
		rlPass, _ := cmd.Flags().GetString("request-log-postgres-password")
		failOpen, _ := cmd.Flags().GetBool("fail-open")
		otelEndpoint, _ := cmd.Flags().GetString("otel-endpoint")
		otelProtocol, _ := cmd.Flags().GetString("otel-protocol")
		otelSample, _ := cmd.Flags().GetFloat64("otel-sample")
		patternsFile, _ := cmd.Flags().GetString("standin-patterns-file")
		openaiBase, _ := cmd.Flags().GetString("openai-base-url")
		modelsFile, _ := cmd.Flags().GetString("models-file")
		ptSecret, _ := cmd.Flags().GetString("passthrough-secret")
		ingressHost, _ := cmd.Flags().GetString("ingress-host")
		enablePDB, _ := cmd.Flags().GetBool("enable-pdb")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		imgAGW, _ := cmd.Flags().GetString("image-agentgateway")
		imgStandIn, _ := cmd.Flags().GetString("image-standin")
		extProcTimeout, _ := cmd.Flags().GetString("extproc-timeout")
		standinCPURequest, _ := cmd.Flags().GetString("standin-cpu-request")
		standinCPULimit, _ := cmd.Flags().GetString("standin-cpu-limit")
		imgPinball, _ := cmd.Flags().GetString("image-pinball")
		imgCopilot, _ := cmd.Flags().GetString("image-copilot-api")

		opts := &k8s.ClusterOptions{
			Namespace:            ns,
			Provider:             prov,
			CopilotToken:         copilotTok,
			OpenAIKey:            openaiKey,
			AnthropicKey:         anthropicKey,
			OptSchedule:          optSchedule,
			OptimizerDSN:         optDSN,
			RequestLogDB:         rlDB,
			RequestLogPostgres:   rlPG,
			RequestLogPGPassword: rlPass,
			FailOpen:             failOpen,
			OTel:                 config.OTel{Endpoint: otelEndpoint, Protocol: otelProtocol, Sample: otelSample},
			StandinPatternsFile:  patternsFile,
			OpenAIBaseURL:        openaiBase,
			ModelsFile:           modelsFile,
			PassthroughSecret:    ptSecret,
			IngressHost:          ingressHost,
			EnablePDB:            enablePDB,
			DryRun:               dryRun,
			ImageAgentGateway:    imgAGW,
			ImageStandIn:         imgStandIn,
			ImagePinball:         imgPinball,
			ImageCopilotAPI:      imgCopilot,
			ExtProcTimeout:       extProcTimeout,
			StandinCPURequest:    standinCPURequest,
			StandinCPULimit:      standinCPULimit,
		}

		k8s.ReuseExistingSecrets(opts)
		if prov == "copilot" && opts.CopilotToken == "" {
			return fmt.Errorf("--provider copilot needs --copilot-token (no existing convergence-copilot Secret in namespace %q to reuse)", ns)
		}

		manifests, err := k8s.RenderManifests(opts)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Println(manifests)
			return nil
		}

		if noBackup, _ := cmd.Flags().GetBool("no-backup"); !noBackup {
			if err := autoBackup(ns); err != nil {
				return err
			}
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

var clusterLogsCmd = &cobra.Command{
	Use:       "logs [service]",
	Short:     "Show logs for in-cluster pods (agentgateway, stand-in, pinball, copilot-api)",
	ValidArgs: []string{"agentgateway", "stand-in", "pinball", "copilot-api"},
	Args:      cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetString("tail")

		kArgs := []string{"-n", ns, "logs"}
		if len(args) > 0 {
			kArgs = append(kArgs, "-l", "app="+args[0])
		} else {
			kArgs = append(kArgs, "-l", "app in (agentgateway, stand-in, pinball, copilot-api)")
		}
		if follow {
			kArgs = append(kArgs, "-f")
		}
		if tail != "" {
			kArgs = append(kArgs, "--tail="+tail)
		}
		c := exec.CommandContext(cmd.Context(), "kubectl", kArgs...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()
		if err := c.Run(); err != nil {
			return fmt.Errorf("kubectl logs failed: %w", err)
		}
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
	clusterInstallCmd.Flags().String("otel-endpoint", "", "Export OTLP traces to this collector host:port (default: off)")
	clusterInstallCmd.Flags().String("otel-protocol", "", "OTLP protocol: grpc (default) or http")
	clusterInstallCmd.Flags().Float64("otel-sample", 0, "Trace sampling rate 0..1 (default 1)")
	clusterInstallCmd.Flags().String("request-log-db", "", "Postgres DSN for agentgateway's request log (llm: full, STORES PROMPTS after Stand-In scrubbing); also enables the optimizer CronJob. Stored in Secret pinball-optimizer-db")
	clusterInstallCmd.Flags().Bool("request-log-postgres", false, "Run a small Postgres (StatefulSet, 5Gi PVC) in the namespace as the request log; same effect as --request-log-db")
	clusterInstallCmd.Flags().String("request-log-postgres-password", "", "Password for --request-log-postgres (default: reuse the existing Secret, else generate)")
	clusterInstallCmd.Flags().Bool("fail-open", false, "If Stand-In is down, let requests through anyway: they then go UNSCRUBBED to the upstream (default: fail closed, reject them)")
	clusterInstallCmd.Flags().String("openai-base-url", "", "Base URL of an OpenAI-compatible endpoint for provider=direct (testing / self-hosted)")
	_ = clusterInstallCmd.Flags().MarkHidden("openai-base-url")
	clusterInstallCmd.Flags().String("passthrough-secret", "convergence-passthrough", "Existing Secret in the namespace whose keys are the passthrough api_key_env names")
	clusterInstallCmd.Flags().String("standin-patterns-file", "", "Stand-In patterns.yaml (replaces the image default; rendered into ConfigMap stand-in-patterns)")
	clusterInstallCmd.Flags().String("models-file", "", "models.yaml override file (same schema as ~/.convergence/models.yaml) for the tier->upstream model mapping")
	clusterInstallCmd.Flags().String("ingress-host", "", "Host for an optional Ingress resource pointing to agentgateway:3000")
	clusterInstallCmd.Flags().Bool("enable-pdb", false, "Render PodDisruptionBudgets (minAvailable: 1) for gateway, stand-in, and pinball")
	clusterInstallCmd.Flags().Bool("dry-run", false, "Output rendered YAML to stdout without applying")
	clusterInstallCmd.Flags().String("image-agentgateway", "", "Override agentgateway image (tag or digest)")
	clusterInstallCmd.Flags().String("extproc-timeout", "", "ext_proc requestTimeout as a Go duration (default 5s)")
	clusterInstallCmd.Flags().String("standin-cpu-request", "", "Stand-In CPU request (default 50m)")
	clusterInstallCmd.Flags().String("standin-cpu-limit", "", "Stand-In CPU limit (default 500m); raise if ext_proc times out under load")
	clusterInstallCmd.Flags().String("image-standin", "", "Override stand-in image (tag or digest)")
	clusterInstallCmd.Flags().String("image-pinball", "", "Override pinball image (tag or digest)")
	clusterInstallCmd.Flags().String("image-copilot-api", "", "Override copilot-api image (tag or digest)")

	clusterStatusCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")

	clusterLogsCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")
	clusterLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	clusterLogsCmd.Flags().String("tail", "", "Lines of recent log file to display")

	clusterUninstallCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")
	clusterUninstallCmd.Flags().Bool("yes", false, "Confirm deletion")

	clusterCmd.AddCommand(clusterInstallCmd)
	clusterCmd.AddCommand(clusterUninstallCmd)
	clusterCmd.AddCommand(clusterStatusCmd)
	clusterCmd.AddCommand(clusterLogsCmd)
	rootCmd.AddCommand(clusterCmd)
}
