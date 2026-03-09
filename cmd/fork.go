package cmd

import (
	"fmt"
	"os"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
)

// forkCmd creates a snapshot of a VM, then creates a new VM from that snapshot.
var forkCmd = &cobra.Command{
	Use:   "fork <vm-id-or-name>",
	Short: "Fork a VM by snapshotting and creating a new VM from it",
	Long: `Fork a VM by creating a snapshot and then spinning up a new VM from it.

This is a convenience command that combines "snapshots create --wait" and
"create --snapshot" into a single step. The snapshot is taken with zero
downtime — the source VM is not affected.

Examples:
  irons fork my-dev-env --fork-name my-dev-env-copy
  irons fork vm_k3mf9xvw2p --fork-name experiment
  irons fork my-dev-env --fork-name copy --key ~/.ssh/id_rsa.pub`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		forkName, _ := cmd.Flags().GetString("fork-name")
		keyPath, _ := cmd.Flags().GetString("key")

		keyContent, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("reading SSH key file %s: %w", keyPath, err)
		}

		client := newClient()

		vmID, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		// Step 1: Create a snapshot and wait for it to be ready.
		fmt.Printf("Creating snapshot of VM '%s'...\n", idOrName)
		snap, err := client.SnapshotsCreate(vmID, api.CreateSnapshotRequest{})
		if err != nil {
			return fmt.Errorf("creating snapshot: %w", err)
		}

		if err := waitForSnapshot(cmd.Context(), client, snap.ID); err != nil {
			return err
		}
		fmt.Printf("✓ Snapshot ready: %s\n", snap.ID)

		// Step 2: Create a new VM from the snapshot.
		fmt.Printf("Creating VM '%s' from snapshot...\n", forkName)
		vm, err := client.Create(keyContent, forkName, snap.ID)
		if err != nil {
			return fmt.Errorf("creating VM from snapshot: %w", err)
		}

		fmt.Printf("✓ VM created: %s\n", vm.ID)

		if err := waitForVMCond(cmd.Context(), client, vm.ID, statusAndDetailEq("running", "ready")); err != nil {
			return err
		}

		fmt.Printf("✓ VM '%s' is ready!\n", forkName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(forkCmd)

	forkCmd.Flags().String("fork-name", "", "Name for the forked VM (required)")
	forkCmd.MarkFlagRequired("fork-name")

	forkCmd.Flags().StringP("key", "k", defaultSSHKeyPath(), "SSH public key path")
}
