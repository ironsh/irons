package cmd

import (
	"fmt"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop NAME",
	Short: "Stop a sandbox",
	Long: `Stop a running sandbox.

This command powers off the specified sandbox. The sandbox
can be restarted later with the start command.

By default the command waits until the sandbox is stopped before
returning. Pass --async to return immediately after the stop
request is accepted.

Examples:
  irons stop my-sandbox
  irons stop --async my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		async, _ := cmd.Flags().GetBool("async")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Stopping sandbox '%s'...\n", name)

		if err := client.Stop(name); err != nil {
			return fmt.Errorf("stopping sandbox: %w", err)
		}

		if async {
			fmt.Printf("✓ Stop request accepted for sandbox '%s'.\n", name)
			return nil
		}

		if err := waitForStatus(client, name, []string{"stopped"}); err != nil {
			return err
		}

		fmt.Printf("✓ Sandbox '%s' stopped successfully!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().Bool("async", false, "Return immediately without waiting for the sandbox to reach the stopped state")
}
