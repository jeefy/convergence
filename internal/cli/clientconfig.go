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

func getGatewayURL() (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("http://localhost:%d/v1", cfg.GatewayPort), nil
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

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureOpenCode(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated OpenCode provider endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to opencode.json.convergence.bak")
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

var configClaudeCodeCmd = &cobra.Command{
	Use:   "claude-code [restore]",
	Short: "Point Claude Code (~/.claude/settings.json) to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureClaudeCode("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Claude Code settings from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureClaudeCode(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated Claude Code Anthropic endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to settings.json.convergence.bak")
		return nil
	},
}

var configAiderCmd = &cobra.Command{
	Use:   "aider [restore]",
	Short: "Point Aider (~/.aider.conf.yml) to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureAider("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Aider configuration from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureAider(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated Aider OpenAI endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to .aider.conf.yml.convergence.bak")
		return nil
	},
}

var configHermesCmd = &cobra.Command{
	Use:   "hermes [restore]",
	Short: "Point Hermes (~/.hermes/config.yaml) to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureHermes("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Hermes configuration from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureHermes(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated Hermes endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to config.yaml.convergence.bak")
		return nil
	},
}

var configGooseCmd = &cobra.Command{
	Use:   "goose [restore]",
	Short: "Point Goose (~/.config/goose/config.yaml) to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureGoose("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Goose configuration from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureGoose(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated Goose OpenAI endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to config.yaml.convergence.bak")
		return nil
	},
}

var configCursorCmd = &cobra.Command{
	Use:   "cursor [restore]",
	Short: "Point Cursor settings to Convergence (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureCursor("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Cursor settings from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureCursor(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully updated Cursor OpenAI endpoint to: %s\n", gwURL)
		fmt.Println("Backup saved to settings.json.convergence.bak")
		return nil
	},
}

var configContinueCmd = &cobra.Command{
	Use:   "continue [restore]",
	Short: "Add Convergence to Continue.dev (~/.continue/config.json) (or restore original)",
	RunE: func(cmd *cobra.Command, args []string) error {
		restore := len(args) > 0 && args[0] == "restore"
		if restore {
			if err := clientconfig.ConfigureContinue("", true); err != nil {
				return err
			}
			fmt.Println("Successfully restored Continue.dev configuration from backup.")
			return nil
		}

		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		if err := clientconfig.ConfigureContinue(gwURL, false); err != nil {
			return err
		}
		fmt.Printf("Successfully added Convergence to Continue.dev models pointing to: %s\n", gwURL)
		fmt.Println("Backup saved to config.json.convergence.bak")
		return nil
	},
}

var configCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Print environment variables to point Copilot/OpenAI tools to Convergence",
	RunE: func(cmd *cobra.Command, args []string) error {
		gwURL, err := getGatewayURL()
		if err != nil {
			return err
		}
		fmt.Println(clientconfig.PrintCopilotEnv(gwURL))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configOpenCodeCmd)
	configCmd.AddCommand(configOmoCmd)
	configCmd.AddCommand(configClaudeCodeCmd)
	configCmd.AddCommand(configAiderCmd)
	configCmd.AddCommand(configHermesCmd)
	configCmd.AddCommand(configGooseCmd)
	configCmd.AddCommand(configCursorCmd)
	configCmd.AddCommand(configContinueCmd)
	configCmd.AddCommand(configCopilotCmd)
	rootCmd.AddCommand(configCmd)
}
