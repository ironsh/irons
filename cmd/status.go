package cmd

import (
	"fmt"
	"strings"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status ID",
	Short: "Show status of a VM",
	Long: `Show the current status and health of a specific VM.

This command allows you to view detailed information about
the state and configuration of a VM.

Examples:
  irons status vm_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Make API call
		resp, err := client.GetVM(id)
		if err != nil {
			return fmt.Errorf("getting VM status: %w", err)
		}

		// Show status information
		fmt.Printf("\n✓ VM Status:\n")
		fmt.Printf("  ID: %s\n", resp.ID)
		fmt.Printf("  Name: %s\n", resp.Name)
		fmt.Printf("  Status: %s\n", resp.Status)
		if resp.StatusDetail != "" {
			fmt.Printf("  Detail: %s\n", resp.StatusDetail)
		}
		fmt.Printf("  Created: %s\n", resp.CreatedAt)
		fmt.Printf("  Updated: %s\n", resp.UpdatedAt)

		// Add visual status indicator
		status := strings.ToLower(resp.Status)
		switch {
		case status == "running":
			fmt.Printf("\n🟢 VM is healthy and ready\n")
		case status == "creating" || status == "starting":
			fmt.Printf("\n🟡 VM is starting up\n")
		case status == "stopped" || status == "stopping":
			fmt.Printf("\n🟠 VM is stopped\n")
		case status == "failed":
			fmt.Printf("\n🔴 VM has errors\n")
		default:
			fmt.Printf("\n⚪ VM status: %s\n", resp.Status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
