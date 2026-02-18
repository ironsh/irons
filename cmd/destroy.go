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

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy NAME",
	Short: "Destroy a sandbox",
	Long: `Destroy a sandbox and clean up associated components.

This command allows you to safely destroy a specific sandbox
and remove it from the system with proper cleanup.

Examples:
  irons destroy my-sandbox
  irons destroy prod-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're destroying
		fmt.Printf("Destroying sandbox '%s'...\n", name)

		// Make API call
		if _, err := client.Destroy(name); err != nil {
			return fmt.Errorf("destroying sandbox: %w", err)
		}

		// Show success
		fmt.Printf("✓ Sandbox destroyed successfully!\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
