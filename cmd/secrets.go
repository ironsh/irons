package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ironsh/irons/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// secretsCmd represents the secrets command group
var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets",
	Long: `Manage secrets for your account.

Secrets are credentials (e.g. API tokens) that are securely stored and
made available to VMs as environment variables via proxy values.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// secretsListCmd lists all secrets
var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets",
	Long: `List all secrets on the account.

Displays a table with name, provider, env var, proxy value, and creation date.
Never displays the secret value.

Examples:
  irons secrets list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		resp, err := client.SecretsList()
		if err != nil {
			return fmt.Errorf("listing secrets: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No secrets found.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"Name", "Provider", "Env Var", "Proxy Value", "Created"})
		for _, s := range resp.Data {
			table.Append([]string{s.Name, s.Provider, s.EnvVar, s.ProxyValue, s.CreatedAt})
		}
		table.Render()

		return nil
	},
}

// secretsAddCmd adds a new secret
var secretsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new secret",
	Long: `Add a new secret to your account.

If --secret is not provided, the CLI will prompt for the value interactively
with echo disabled, or read from stdin if input is piped.

Examples:
  irons secrets add --name github-main --provider github --env-var GITHUB_TOKEN --secret ghp_abc123
  irons secrets add --name github-main --provider github --env-var GITHUB_TOKEN`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		provider, _ := cmd.Flags().GetString("provider")
		envVar, _ := cmd.Flags().GetString("env-var")
		secret, _ := cmd.Flags().GetString("secret")
		comment, _ := cmd.Flags().GetString("comment")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if provider == "" {
			return fmt.Errorf("--provider is required")
		}
		if provider != "github" && provider != "npm" {
			return fmt.Errorf("unsupported provider %q (must be github or npm)", provider)
		}
		if envVar == "" {
			return fmt.Errorf("--env-var is required")
		}

		if secret == "" {
			var err error
			secret, err = readSecret()
			if err != nil {
				return err
			}
		}

		client := newClient()

		req := api.CreateSecretRequest{
			Name:     name,
			Provider: provider,
			Secret:   secret,
			EnvVar:   envVar,
			Comment:  comment,
		}

		s, err := client.SecretsCreate(req)
		if err != nil {
			return fmt.Errorf("creating secret: %w", err)
		}

		printSecretDetail(s)
		return nil
	},
}

// secretsRemoveCmd removes a secret
var secretsRemoveCmd = &cobra.Command{
	Use:   "remove <name|id>",
	Short: "Remove a secret",
	Long: `Remove a secret by name or ID.

If the value starts with "sec_", it is treated as an ID. Otherwise it is
treated as a name and resolved via the API.

Examples:
  irons secrets remove github-main
  irons secrets remove sec_m4xk9wp2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]

		client := newClient()

		id, err := resolveSecret(client, idOrName)
		if err != nil {
			return err
		}

		if err := client.SecretsDelete(id); err != nil {
			return fmt.Errorf("removing secret: %w", err)
		}

		fmt.Printf("Secret %q removed.\n", idOrName)
		return nil
	},
}

// secretsUpdateCmd updates a secret
var secretsUpdateCmd = &cobra.Command{
	Use:   "update <name|id>",
	Short: "Update a secret",
	Long: `Update a secret's value, env var, or comment.

If --secret is not provided and no other flags are set, the CLI will prompt
for a new secret value interactively.

Examples:
  irons secrets update github-main --secret ghp_newtoken789
  irons secrets update github-main --env-var GH_TOKEN
  irons secrets update github-main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		secret, _ := cmd.Flags().GetString("secret")
		envVar, _ := cmd.Flags().GetString("env-var")
		comment, _ := cmd.Flags().GetString("comment")

		secretSet := cmd.Flags().Changed("secret")
		envVarSet := cmd.Flags().Changed("env-var")
		commentSet := cmd.Flags().Changed("comment")

		// If no flags at all, prompt for secret value
		if !secretSet && !envVarSet && !commentSet {
			var err error
			secret, err = readSecret()
			if err != nil {
				return err
			}
			secretSet = true
		}

		client := newClient()

		id, err := resolveSecret(client, idOrName)
		if err != nil {
			return err
		}

		req := api.UpdateSecretRequest{}
		if secretSet {
			req.Secret = secret
		}
		if envVarSet {
			req.EnvVar = envVar
		}
		if commentSet {
			req.Comment = comment
		}

		s, err := client.SecretsUpdate(id, req)
		if err != nil {
			return fmt.Errorf("updating secret: %w", err)
		}

		printSecretDetail(s)
		return nil
	},
}

// secretsShowCmd shows details of a single secret
var secretsShowCmd = &cobra.Command{
	Use:   "show <name|id>",
	Short: "Show details of a secret",
	Long: `Show details of a single secret by name or ID.

Displays the secret's metadata. Never displays the secret value.

Examples:
  irons secrets show github-main
  irons secrets show sec_m4xk9wp2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]

		client := newClient()

		id, err := resolveSecret(client, idOrName)
		if err != nil {
			return err
		}

		s, err := client.SecretsGet(id)
		if err != nil {
			return fmt.Errorf("getting secret: %w", err)
		}

		printSecretDetail(s)
		return nil
	},
}

func printSecretDetail(s *api.Secret) {
	fmt.Printf("\n✓ Secret:\n")
	fmt.Printf("  ID:          %s\n", s.ID)
	fmt.Printf("  Name:        %s\n", s.Name)
	fmt.Printf("  Provider:    %s\n", s.Provider)
	fmt.Printf("  Env Var:     %s\n", s.EnvVar)
	fmt.Printf("  Proxy Value: %s\n", s.ProxyValue)
	if s.Comment != nil && *s.Comment != "" {
		fmt.Printf("  Comment:     %s\n", *s.Comment)
	}
	fmt.Printf("  Created:     %s\n", s.CreatedAt)
	fmt.Printf("  Updated:     %s\n", s.UpdatedAt)
}

// resolveSecret resolves a secret name or ID to a secret ID.
func resolveSecret(client *api.Client, idOrName string) (string, error) {
	id, err := client.ResolveSecret(idOrName)
	if err != nil {
		return "", fmt.Errorf("resolving secret %q: %w", idOrName, err)
	}
	return id, nil
}

// readSecret reads a secret value from stdin. If stdin is a terminal, it
// prompts interactively with echo disabled and requires confirmation. If
// stdin is piped, it reads a single line.
func readSecret() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Piped input: read a single line
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("reading secret from stdin: %w", err)
			}
			return "", fmt.Errorf("no input provided on stdin")
		}
		return strings.TrimSpace(scanner.Text()), nil
	}

	// Interactive: prompt with echo disabled
	fmt.Fprint(os.Stderr, "Enter secret value: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading secret: %w", err)
	}

	fmt.Fprint(os.Stderr, "Confirm secret value: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading secret confirmation: %w", err)
	}

	if string(first) != string(second) {
		return "", fmt.Errorf("secret values do not match")
	}

	value := strings.TrimSpace(string(first))
	if value == "" {
		return "", fmt.Errorf("secret value cannot be empty")
	}

	return value, nil
}

func init() {
	rootCmd.AddCommand(secretsCmd)

	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsAddCmd)
	secretsCmd.AddCommand(secretsRemoveCmd)
	secretsCmd.AddCommand(secretsUpdateCmd)
	secretsCmd.AddCommand(secretsShowCmd)

	// Flags for add command
	secretsAddCmd.Flags().String("name", "", "Human-readable name for the secret")
	secretsAddCmd.Flags().String("provider", "", "Provider (github, npm)")
	secretsAddCmd.Flags().String("env-var", "", "Environment variable name for VMs")
	secretsAddCmd.Flags().String("secret", "", "The secret value (prompts if omitted)")
	secretsAddCmd.Flags().String("comment", "", "Optional note")

	// Flags for update command
	secretsUpdateCmd.Flags().String("secret", "", "New secret value (prompts if omitted)")
	secretsUpdateCmd.Flags().String("env-var", "", "Updated environment variable name")
	secretsUpdateCmd.Flags().String("comment", "", "Updated comment")
}
