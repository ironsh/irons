package cmd

import (
	"fmt"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start ID",
	Short: "Start a VM",
	Long: `Start a VM that has been previously stopped.

This command powers on the specified VM and makes it
available for use again.

By default the command waits until the VM is running before
returning. Pass --async to return immediately after the start
request is accepted.

Examples:
  irons start vm_abc123
  irons start --async vm_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		async, _ := cmd.Flags().GetBool("async")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Starting VM '%s'...\n", id)

		if _, err := client.Start(id); err != nil {
			return fmt.Errorf("starting VM: %w", err)
		}

		if async {
			fmt.Printf("✓ Start request accepted for VM '%s'.\n", id)
			return nil
		}

		if err := waitForStatus(cmd.Context(), client, id, []string{"running"}); err != nil {
			return err
		}

		fmt.Printf("✓ VM '%s' started successfully!\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().Bool("async", false, "Return immediately without waiting for the VM to reach the running state")
}
