package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jeefy/convergence/internal/config"
	"github.com/jeefy/convergence/internal/engine"
	"github.com/jeefy/convergence/internal/templates"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Convergence AI Gateway containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}

		// Ensure rendered assets exist
		if err := templates.RenderAssets(cfg, dir); err != nil {
			return err
		}

		eng, err := engine.DetectEngine(cfg.Engine)
		if err != nil {
			return err
		}

		fmt.Printf("Starting Convergence services with %s...\n", eng.Name())
		ctx := context.Background()
		if err := eng.Start(ctx, dir); err != nil {
			return err
		}

		fmt.Println("Services started. Verifying gateway health...")
		time.Sleep(2 * time.Second)

		healthCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		if err := eng.CheckHealth(healthCtx, cfg.GatewayPort); err != nil {
			fmt.Printf("Gateway starting up (health check pending: %v)\n", err)
		} else {
			fmt.Println("AgentGateway is healthy and ready!")
		}

		fmt.Printf("\nBase URL: http://localhost:%d/v1\n", cfg.GatewayPort)
		fmt.Printf("API Key:  carrier (or any non-empty string)\n")
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Convergence AI Gateway containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		dir, err := config.GetConfigDir()
		if err != nil {
			return err
		}
		eng, err := engine.DetectEngine(cfg.Engine)
		if err != nil {
			return err
		}

		fmt.Printf("Stopping Convergence services with %s...\n", eng.Name())
		return eng.Stop(context.Background(), dir)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of Convergence services",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		eng, err := engine.DetectEngine(cfg.Engine)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		fmt.Println("=== Convergence Status ===")
		fmt.Printf("Container Engine: %s\n", eng.Name())
		fmt.Printf("Active Provider:  %s\n", cfg.Provider)
		fmt.Printf("Gateway Endpoint: http://localhost:%d/v1\n", cfg.GatewayPort)

		if err := eng.CheckHealth(ctx, cfg.GatewayPort); err != nil {
			fmt.Printf("Health:           DOWN (%v)\n", err)
		} else {
			fmt.Printf("Health:           ONLINE (200 OK)\n")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
}
