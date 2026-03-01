package cmd

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
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
		client := newClient()

		// Make API call
		resp, err := client.ListVMs()
		if err != nil {
			return fmt.Errorf("listing VMs: %w", err)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No VMs found.")
			return nil
		}

		hasDetail := false
		for _, vm := range resp.Data {
			if vm.StatusDetail != "" {
				hasDetail = true
				break
			}
		}

		table := tablewriter.NewTable(os.Stdout)
		if hasDetail {
			table.Header([]string{"Name", "ID", "Status", "Status Detail", "Created At"})
			for _, vm := range resp.Data {
				table.Append([]string{vm.Name, vm.ID, vm.Status, vm.StatusDetail, vm.CreatedAt})
			}
		} else {
			table.Header([]string{"Name", "ID", "Status", "Created At"})
			for _, vm := range resp.Data {
				table.Append([]string{vm.Name, vm.ID, vm.Status, vm.CreatedAt})
			}
		}
		table.Render()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
