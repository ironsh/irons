package cmd

import (
	"fmt"
	"strings"

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

Use --force to automatically stop the sandbox first if it is
currently running.

Examples:
  irons destroy my-sandbox
  irons destroy --force my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		if force {
			// Check current status before deciding whether to stop first.
			statusResp, err := client.Status(name)
			if err != nil {
				return fmt.Errorf("getting sandbox status: %w", err)
			}

			current := strings.ToLower(statusResp.Status)
			if strings.Contains(current, "running") || strings.Contains(current, "ready") {
				fmt.Printf("Stopping sandbox '%s' before destroying...\n", name)

				if err := client.Stop(name); err != nil {
					return fmt.Errorf("stopping sandbox: %w", err)
				}

				if err := waitForStatus(client, name, []string{"stopped"}); err != nil {
					return err
				}

				fmt.Printf("✓ Sandbox '%s' stopped.\n", name)
			}
		}

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
	destroyCmd.Flags().Bool("force", false, "Stop the sandbox first if it is currently running")
}
