package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new sandbox",
	Long: `Create a new sandbox with the specified configuration.

This command allows you to create a new sandbox with SSH key,
secrets, and custom naming options.

By default the command waits until the sandbox is running before
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
  irons create my-sandbox
  irons create --async my-sandbox
  irons create --key ~/.ssh/my_key.pub my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyPath, _ := cmd.Flags().GetString("key")
		name := args[0]
		async, _ := cmd.Flags().GetBool("async")

		// Read SSH key file
		keyContent, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("reading SSH key file %s: %w", keyPath, err)
		}

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're creating
		fmt.Printf("Creating sandbox '%s'...\n", name)

		// Make API call
		resp, err := client.Create(keyContent, name)
		if err != nil {
			return fmt.Errorf("creating sandbox: %w", err)
		}

		// Show initial response
		fmt.Printf("✓ Sandbox created successfully!\n")
		fmt.Printf("  ID: %s\n", resp.ID)
		fmt.Printf("  Name: %s\n", resp.Name)
		fmt.Printf("  Status: %s\n", resp.Status)

		if async {
			return nil
		}

		if err := waitForStatus(client, name, []string{"ready"}); err != nil {
			return err
		}

		fmt.Printf("✓ Sandbox '%s' is ready!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Get default SSH key path with error handling
	// Prefer ed25519, fall back to RSA, then empty string
	defaultKeyPath := "" // fallback default
	if homeDir, err := os.UserHomeDir(); err == nil {
		rsaKey := filepath.Join(homeDir, ".ssh", "id_rsa.pub")
		ed25519Key := filepath.Join(homeDir, ".ssh", "id_ed25519.pub")

		ecdsaKey := filepath.Join(homeDir, ".ssh", "id_ecdsa.pub")
		ecdsaSkKey := filepath.Join(homeDir, ".ssh", "id_ecdsa_sk.pub")
		ed25519SkKey := filepath.Join(homeDir, ".ssh", "id_ed25519_sk.pub")

		switch {
		case fileExists(ed25519Key):
			defaultKeyPath = ed25519Key
		case fileExists(ed25519SkKey):
			defaultKeyPath = ed25519SkKey
		case fileExists(ecdsaKey):
			defaultKeyPath = ecdsaKey
		case fileExists(ecdsaSkKey):
			defaultKeyPath = ecdsaSkKey
		case fileExists(rsaKey):
			defaultKeyPath = rsaKey
		default:
			defaultKeyPath = ed25519Key // sensible default path even if absent
		}
	}

	// Define flags
	createCmd.Flags().StringP("key", "k", defaultKeyPath, "SSH public key path")
	createCmd.Flags().Bool("async", false, "Return immediately without waiting for the sandbox to reach the running state")
}

// fileExists returns true if the file at path exists and is accessible.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
