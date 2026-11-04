package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/engine"
	"github.com/jeefy/convergence/internal/templates"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup for Convergence AI Gateway",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("=== Convergence Setup Wizard ===")

		// 1. Detect engine
		eng, err := engine.DetectEngine("")
		if err != nil {
			return fmt.Errorf("container engine check: %w", err)
		}
		fmt.Printf("Detected container engine: %s\n\n", eng.Name())

		cfg, err := config.LoadConfig()
		if err != nil {
			cfg = config.DefaultConfig()
		}
		cfg.Engine = eng.Name()

		// 2. Select Provider
		fmt.Println("Select your primary LLM backend:")
		fmt.Println("  [1] GitHub Copilot (Subscription token)")
		fmt.Println("  [2] Direct API Keys (OpenAI / Anthropic / Gemini)")
		fmt.Println("  [3] Local Only (Ollama)")
		fmt.Print("Enter choice [1-3] (default 1): ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		if choice == "" {
			choice = "1"
		}

		switch choice {
		case "1":
			cfg.Provider = config.ProviderCopilot
			fmt.Print("Enter GitHub Copilot Token (or press Enter to skip for now): ")
			tok, _ := reader.ReadString('\n')
			cfg.CopilotToken = strings.TrimSpace(tok)
		case "2":
			cfg.Provider = config.ProviderDirect
			fmt.Print("Enter Anthropic API Key (or press Enter to skip): ")
			ant, _ := reader.ReadString('\n')
			cfg.AnthropicKey = strings.TrimSpace(ant)

			fmt.Print("Enter OpenAI API Key (or press Enter to skip): ")
			oai, _ := reader.ReadString('\n')
			cfg.OpenAIKey = strings.TrimSpace(oai)
		case "3":
			cfg.Provider = config.ProviderOllama
			fmt.Print("Enter Ollama URL (default http://host.docker.internal:11434): ")
			url, _ := reader.ReadString('\n')
			url = strings.TrimSpace(url)
			if url != "" {
				cfg.OllamaURL = url
			}
		}

		// Save config
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}

		// Render compose & gateway assets
		if err := templates.RenderAssets(cfg, dir); err != nil {
			return fmt.Errorf("rendering templates: %w", err)
		}

		fmt.Println("\nConfiguration rendered successfully to:", dir)
		fmt.Println("Run 'convergence start' to launch your gateway.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
