package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new VM",
	Long: `Create a new VM with the specified configuration.

This command allows you to create a new VM with SSH key,
secrets, and custom naming options.

By default the command waits until the VM is running before
returning. Pass --async to return immediately after the create
request is accepted.

SSH Key Detection:
  If --key is not provided, the following key files are checked in
  order and the first one found is used:
    ~/.ssh/id_ed25519.pub
    ~/.ssh/id_ed25519_sk.pub
    ~/.ssh/id_ecdsa.pub
    ~/.ssh/id_ecdsa_sk.pub
    ~/.ssh/id_rsa.pub

Examples:
  irons create my-vm
  irons create --async my-vm
  irons create --key ~/.ssh/my_key.pub my-vm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyPath, _ := cmd.Flags().GetString("key")
		name := args[0]
		async, _ := cmd.Flags().GetBool("async")
		snapshotID, _ := cmd.Flags().GetString("snapshot")

		// Read SSH key file
		keyContent, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("reading SSH key file %s: %w", keyPath, err)
		}

		// Create API client
		client := newClient()

		// Show what we're creating
		fmt.Printf("Creating VM '%s'...\n", name)

		// Make API call
		resp, err := client.Create(keyContent, name, snapshotID)
		if err != nil {
			return fmt.Errorf("creating VM: %w", err)
		}

		// Show initial response
		fmt.Printf("✓ VM created successfully!\n")
		fmt.Printf("  ID: %s\n", resp.ID)
		fmt.Printf("  Name: %s\n", resp.Name)
		fmt.Printf("  Status: %s\n", resp.Status)
		if resp.StatusDetail != "" {
			fmt.Printf("  Detail: %s\n", resp.StatusDetail)
		}

		if async {
			return nil
		}

		if err := waitForVMCond(cmd.Context(), client, resp.ID, statusAndDetailEq("running", "ready")); err != nil {
			return err
		}

		fmt.Printf("✓ VM '%s' is ready!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringP("key", "k", defaultSSHKeyPath(), "SSH public key path")
	createCmd.Flags().Bool("async", false, "Return immediately without waiting for the VM to reach the running state")
	createCmd.Flags().String("snapshot", "", "Restore from a snapshot ID")
}

// defaultSSHKeyPath returns the path to the first SSH public key found in
// ~/.ssh, checking in order: ed25519, ed25519_sk, ecdsa, ecdsa_sk, rsa.
// Falls back to ~/.ssh/id_ed25519.pub if none are found.
func defaultSSHKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519.pub"),
		filepath.Join(homeDir, ".ssh", "id_ed25519_sk.pub"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa.pub"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa_sk.pub"),
		filepath.Join(homeDir, ".ssh", "id_rsa.pub"),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return candidates[0] // sensible default path even if absent
}

// fileExists returns true if the file at path exists and is accessible.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
