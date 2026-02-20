package cmd

import (
	"fmt"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start NAME",
	Short: "Start a sandbox",
	Long: `Start a sandbox that has been previously stopped.

This command powers on the specified sandbox and makes it
available for use again.

By default the command waits until the sandbox is running before
returning. Pass --async to return immediately after the start
request is accepted.

Examples:
  irons start my-sandbox
  irons start --async my-sandbox`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		async, _ := cmd.Flags().GetBool("async")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Starting sandbox '%s'...\n", name)

		if err := client.Start(name); err != nil {
			return fmt.Errorf("starting sandbox: %w", err)
		}

		if async {
			fmt.Printf("✓ Start request accepted for sandbox '%s'.\n", name)
			return nil
		}

		if err := waitForStatus(client, name, []string{"ready"}); err != nil {
			return err
		}

		fmt.Printf("✓ Sandbox '%s' started successfully!\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().Bool("async", false, "Return immediately without waiting for the sandbox to reach the running state")
}
