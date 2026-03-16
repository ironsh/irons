package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ironsh/irons/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// publicKeysCmd represents the public-keys command group
var publicKeysCmd = &cobra.Command{
	Use:   "public-keys",
	Short: "Manage public keys",
	Long: `Manage SSH public keys for your account.

Public keys are used for authenticating SSH connections to your VMs.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// publicKeysListCmd lists all public keys
var publicKeysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all public keys",
	Long: `List all public keys on the account.

Displays a table with name, fingerprint, and creation date.

Examples:
  irons public-keys list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		resp, err := client.PublicKeysList()
		if err != nil {
			return fmt.Errorf("listing public keys: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No public keys found.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"Name", "Fingerprint", "Created"})
		for _, k := range resp.Data {
			table.Append([]string{k.Name, k.Fingerprint, k.CreatedAt})
		}
		table.Render()

		return nil
	},
}

// publicKeysAddCmd adds a new public key
var publicKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new public key",
	Long: `Add a new SSH public key to your account.

The key value can be provided via --public-key, or piped via stdin.

Examples:
  irons public-keys add --name laptop --public-key "ssh-ed25519 AAAA..."
  cat ~/.ssh/id_ed25519.pub | irons public-keys add --name laptop`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		publicKey, _ := cmd.Flags().GetString("public-key")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		if publicKey == "" {
			var err error
			publicKey, err = readPublicKeyFromStdin()
			if err != nil {
				return err
			}
		}

		client := newClient()

		req := api.CreatePublicKeyRequest{
			Name:      name,
			PublicKey: publicKey,
		}

		k, err := client.PublicKeysCreate(req)
		if err != nil {
			return fmt.Errorf("adding public key: %w", err)
		}

		printPublicKeyDetail(k)
		return nil
	},
}

// publicKeysRemoveCmd removes a public key
var publicKeysRemoveCmd = &cobra.Command{
	Use:   "remove <name|id>",
	Short: "Remove a public key",
	Long: `Remove a public key by name or ID.

If the value starts with "pub_", it is treated as an ID. Otherwise it is
treated as a name and resolved via the API.

Examples:
  irons public-keys remove laptop
  irons public-keys remove pub_x9f2km4p`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]

		client := newClient()

		id, err := resolvePublicKey(client, idOrName)
		if err != nil {
			return err
		}

		if err := client.PublicKeysDelete(id); err != nil {
			return fmt.Errorf("removing public key: %w", err)
		}

		fmt.Printf("Public key %q removed.\n", idOrName)
		return nil
	},
}

func printPublicKeyDetail(k *api.PublicKey) {
	fmt.Printf("\n✓ Public Key:\n")
	fmt.Printf("  ID:           %s\n", k.ID)
	fmt.Printf("  Name:         %s\n", k.Name)
	fmt.Printf("  Fingerprint:  %s\n", k.Fingerprint)
	fmt.Printf("  Public Key:   %s\n", k.PublicKey)
	fmt.Printf("  Created:      %s\n", k.CreatedAt)
}

// resolvePublicKey resolves a public key name or ID to a public key ID.
func resolvePublicKey(client *api.Client, idOrName string) (string, error) {
	id, err := client.ResolvePublicKey(idOrName)
	if err != nil {
		return "", fmt.Errorf("resolving public key %q: %w", idOrName, err)
	}
	return id, nil
}

// readPublicKeyFromStdin reads a public key from piped stdin.
func readPublicKeyFromStdin() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading public key from stdin: %w", err)
		}
		return "", fmt.Errorf("no input provided on stdin; use --public-key or pipe key via stdin")
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return "", fmt.Errorf("public key value cannot be empty")
	}
	return value, nil
}

func init() {
	rootCmd.AddCommand(publicKeysCmd)

	publicKeysCmd.AddCommand(publicKeysListCmd)
	publicKeysCmd.AddCommand(publicKeysAddCmd)
	publicKeysCmd.AddCommand(publicKeysRemoveCmd)

	// Flags for add command
	publicKeysAddCmd.Flags().String("name", "", "Human-readable name for the key")
	publicKeysAddCmd.Flags().String("public-key", "", "The SSH public key value (reads from stdin if omitted)")
}
