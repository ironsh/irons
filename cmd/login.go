package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/ironsh/irons/api"
	"github.com/ironsh/irons/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with IronCD",
	Long: `Authenticate with IronCD using device code authorization.

This command initiates a browser-based login flow. You will be given a URL
to visit where you can authorize this device. Once authorized, your API token
will be saved to ~/.config/irons/config.yml automatically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Device auth endpoints live on the console, not the API.
		client := api.NewClient("https://console.iron.sh", "")

		// Step 1: request a device code.
		fmt.Println("Requesting device code...")
		codeResp, err := client.DeviceCode()
		if err != nil {
			return fmt.Errorf("requesting device code: %w", err)
		}

		fmt.Printf("\nOpen the following URL in your browser to authenticate:\n\n  %s\n\n", codeResp.VerificationURI)
		fmt.Printf("Paste in the following device code: %s\n\n", codeResp.Code)
		fmt.Printf("This code expires at %s.\n\n", codeResp.ExpiresAt.Local().Format(time.RFC1123))
		fmt.Println("Waiting for authorization...")

		// Step 2: poll until authorized, expired, or timed out.
		// We use a 1-second interval here regardless of the shared pollInterval.
		deadline := time.Now().Add(pollTimeout)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		ctx := cmd.Context()
		for {
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stderr, "\nLogin cancelled.")
				return nil

			case <-ticker.C:
				if time.Now().After(deadline) {
					return fmt.Errorf("timed out waiting for authorization")
				}

				pollResp, err := client.PollDevice(codeResp.Code)
				if err != nil {
					// Treat transient errors as non-fatal; keep polling.
					fmt.Fprintf(os.Stderr, "warning: poll error (retrying): %v\n", err)
					continue
				}

				switch pollResp.Status {
				case "authorized":
					if err := config.SetAPIKey(pollResp.Token); err != nil {
						return fmt.Errorf("saving token: %w", err)
					}
					fmt.Println("✓ Authorized! Your API token has been saved to ~/.config/irons/config.yml")
					return nil

				case "expired":
					return fmt.Errorf("device code expired — please run `irons login` again")

				case "pending":
					// Still waiting; continue polling.

				default:
					return fmt.Errorf("unexpected poll status %q", pollResp.Status)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
