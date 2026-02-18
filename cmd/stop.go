/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop NAME",
	Short: "Stop a sandbox",
	Long: `Stop a running sandbox.

This command powers off the specified sandbox. The sandbox
can be restarted later with the start command.

Examples:
  irons stop my-sandbox
  irons stop prod-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Stopping sandbox '%s'...\n", name)

		if err := client.Stop(name); err != nil {
			return fmt.Errorf("stopping sandbox: %w", err)
		}

		fmt.Printf("✓ Sandbox '%s' stopped successfully!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
