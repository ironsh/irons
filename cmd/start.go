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

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start NAME",
	Short: "Start a sandbox",
	Long: `Start a sandbox that has been previously stopped.

This command powers on the specified sandbox and makes it
available for use again.

Examples:
  irons start my-sandbox
  irons start prod-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Starting sandbox '%s'...\n", name)

		if err := client.Start(name); err != nil {
			return fmt.Errorf("starting sandbox: %w", err)
		}

		fmt.Printf("✓ Sandbox '%s' started successfully!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
