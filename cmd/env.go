package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// envCmd represents the env command group
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables",
	Long: `Manage environment variables for your account.

Environment variables are injected into VMs at boot.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// envListCmd lists all environment variables
var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environment variables",
	Long: `List all environment variables on the account.

Examples:
  irons env list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		resp, err := client.EnvVarList()
		if err != nil {
			return fmt.Errorf("listing env vars: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No environment variables found.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"Key", "Value", "Updated"})
		for _, e := range resp.Data {
			table.Append([]string{e.Key, e.Value, e.UpdatedAt})
		}
		table.Render()

		return nil
	},
}

// envSetCmd sets an environment variable
var envSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE",
	Short: "Set an environment variable",
	Long: `Set (create or update) an environment variable.

Examples:
  irons env set FOO=BAR
  irons env set DATABASE_URL=postgres://localhost/mydb`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value, ok := strings.Cut(args[0], "=")
		if !ok || key == "" {
			return fmt.Errorf("expected KEY=VALUE format (e.g. FOO=BAR)")
		}

		client := newClient()

		ev, err := client.EnvVarPut(key, value)
		if err != nil {
			return fmt.Errorf("setting env var: %w", err)
		}

		fmt.Printf("Set %s=%s\n", ev.Key, ev.Value)
		return nil
	},
}

// envDestroyCmd deletes an environment variable
var envDestroyCmd = &cobra.Command{
	Use:   "destroy KEY",
	Short: "Delete an environment variable",
	Long: `Delete an environment variable by key.

Examples:
  irons env destroy FOO`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		client := newClient()

		if err := client.EnvVarDelete(key); err != nil {
			return fmt.Errorf("deleting env var: %w", err)
		}

		fmt.Printf("Deleted %s.\n", key)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(envCmd)

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envDestroyCmd)
}
