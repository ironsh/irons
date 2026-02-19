package cmd

import (
	"fmt"
	"strings"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status NAME",
	Short: "Show status of a sandbox",
	Long: `Show the current status and health of a specific sandbox.

This command allows you to view detailed information about
the state and configuration of a sandbox.

Examples:
  irons status my-sandbox
  irons status prod-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're checking
		fmt.Printf("Getting status for sandbox '%s'...\n", name)

		// Make API call
		resp, err := client.Status(name)
		if err != nil {
			return fmt.Errorf("getting sandbox status: %w", err)
		}

		// Show status information
		fmt.Printf("\n✓ Sandbox Status:\n")
		fmt.Printf("  Name: %s\n", resp.Name)
		fmt.Printf("  Status: %s\n", resp.Status)
		fmt.Printf("  Created: %s\n", resp.CreatedAt)
		fmt.Printf("  Updated: %s\n", resp.UpdatedAt)

		if len(resp.Metadata) > 0 {
			fmt.Printf("  Metadata:\n")
			for key, value := range resp.Metadata {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}

		// Add visual status indicator
		status := strings.ToLower(resp.Status)
		switch {
		case strings.Contains(status, "running") || strings.Contains(status, "ready"):
			fmt.Printf("\n🟢 Sandbox is healthy and ready\n")
		case strings.Contains(status, "creating") || strings.Contains(status, "starting"):
			fmt.Printf("\n🟡 Sandbox is starting up\n")
		case strings.Contains(status, "stopped") || strings.Contains(status, "stopping"):
			fmt.Printf("\n🟠 Sandbox is stopped\n")
		case strings.Contains(status, "error") || strings.Contains(status, "failed"):
			fmt.Printf("\n🔴 Sandbox has errors\n")
		default:
			fmt.Printf("\n⚪ Sandbox status: %s\n", resp.Status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
