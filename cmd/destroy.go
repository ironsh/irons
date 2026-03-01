package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy ID",
	Short: "Destroy a VM",
	Long: `Destroy a VM and clean up associated components.

This command allows you to safely destroy a specific VM
and remove it from the system with proper cleanup.

Use --force to automatically stop the VM first if it is
currently running.

Examples:
  irons destroy vm_abc123
  irons destroy --force vm_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		force, _ := cmd.Flags().GetBool("force")

		// Create API client
		client := newClient()

		id, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		if force {
			// Check current status before deciding whether to stop first.
			vm, err := client.GetVM(id)
			if err != nil {
				return fmt.Errorf("getting VM status: %w", err)
			}

			if vm.Status == "running" {
				fmt.Printf("Stopping VM '%s' before destroying...\n", id)

				if _, err := client.Stop(id); err != nil {
					return fmt.Errorf("stopping VM: %w", err)
				}

				if err := waitForVMCond(cmd.Context(), client, id, statusAndDetailEq("stopped", "stopped")); err != nil {
					return err
				}

				fmt.Printf("✓ VM '%s' stopped.\n", id)
			}
		}

		// Show what we're destroying
		fmt.Printf("Destroying VM '%s'...\n", id)

		// Make API call
		if err := client.Destroy(id); err != nil {
			if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "not stopped") {
				return fmt.Errorf("VM must be stopped before destroying. Use --force to stop it first")
			}
			return fmt.Errorf("destroying VM: %w", err)
		}

		// Show success
		fmt.Printf("✓ VM destroyed successfully!\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().Bool("force", false, "Stop the VM first if it is currently running")
}
