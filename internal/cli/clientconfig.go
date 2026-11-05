package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/clientconfig"
	"github.com/jeefy/convergence/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure or restore external client applications to use Convergence",
}

var configOpenCodeCmd = &cobra.Command{
	Use:   "opencode [restore]",
	Short: "Point OpenCode to Convergence (or restore original endpoint)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"

		if restore {
			if err := clientconfig.ConfigureOpenCode("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored OpenCode configuration from backup.")
			return nil
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		gatewayURL := fmt.Sprintf("http://localhost:%d/v1", cfg.GatewayPort)
		if err := clientconfig.ConfigureOpenCode(gatewayURL, false); err != nil {
			return err
		}

		fmt.Printf("Successfully updated OpenCode provider endpoint to: %s\n", gatewayURL)
		fmt.Println("A backup of your previous configuration was saved to opencode.json.convergence.bak")
		return nil
	},
}

var configOmoCmd = &cobra.Command{
	Use:   "oh-my-opencode [restore]",
	Short: "Point oh-my-opencode agent profiles to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"

		if restore {
			if err := clientconfig.ConfigureOmo(true); err != nil {
				return err
			}
			fmt.Println("Successfully restored oh-my-opencode profile from backup.")
			return nil
		}

		if err := clientconfig.ConfigureOmo(false); err != nil {
			return err
		}

		fmt.Println("Successfully configured oh-my-opencode agent profile for Pinball.")
		return nil
	},
}

var configCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Print environment variables to point Copilot/OpenAI tools to Convergence",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		gatewayURL := fmt.Sprintf("http://localhost:%d/v1", cfg.GatewayPort)
		fmt.Println(clientconfig.PrintCopilotEnv(gatewayURL))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configOpenCodeCmd)
	configCmd.AddCommand(configOmoCmd)
	configCmd.AddCommand(configCopilotCmd)
	rootCmd.AddCommand(configCmd)
}
