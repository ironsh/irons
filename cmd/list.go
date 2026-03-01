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
	Short: "List all VMs",
	Long: `List all VMs associated with your account.

This command displays a summary of every VM, including its name,
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
		resp, err := client.ListVMs()
		if err != nil {
			return fmt.Errorf("listing VMs: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No VMs found.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"Name", "ID", "Status", "Created At"})
		for _, vm := range resp.Data {
			table.Append([]string{vm.Name, vm.ID, vm.Status, vm.CreatedAt})
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
