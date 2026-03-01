package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop ID",
	Short: "Stop a VM",
	Long: `Stop a running VM.

This command powers off the specified VM. The VM
can be restarted later with the start command.

By default the command waits until the VM is stopped before
returning. Pass --async to return immediately after the stop
request is accepted.

Examples:
  irons stop vm_abc123
  irons stop --async vm_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		async, _ := cmd.Flags().GetBool("async")

		// Create API client
		client := newClient()

		id, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		fmt.Printf("Stopping VM '%s'...\n", id)

		if _, err := client.Stop(id); err != nil {
			return fmt.Errorf("stopping VM: %w", err)
		}

		if async {
			fmt.Printf("✓ Stop request accepted for VM '%s'.\n", id)
			return nil
		}

		if err := waitForVMCond(cmd.Context(), client, id, statusIn("stopped")); err != nil {
			return err
		}

		fmt.Printf("✓ VM '%s' stopped successfully!\n", id)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().Bool("async", false, "Return immediately without waiting for the VM to reach the stopped state")
}
