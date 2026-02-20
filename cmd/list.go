package cmd

import (
	"fmt"
	"os"

	"github.com/ironsh/irons/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sandboxes",
	Long: `List all sandboxes associated with your account.

This command displays a summary of every sandbox, including its name,
ID, current status, and creation date.

Examples:
  irons list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Make API call
		resp, err := client.List()
		if err != nil {
			return fmt.Errorf("listing sandboxes: %w", err)
		}

		if len(resp.Sandboxes) == 0 {
			fmt.Println("No sandboxes found.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"Name", "ID", "Status", "Created At"})
		for _, s := range resp.Sandboxes {
			table.Append([]string{s.Name, s.ID, s.Status, s.CreatedAt})
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
