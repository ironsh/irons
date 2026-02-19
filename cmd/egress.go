package cmd

import (
	"fmt"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// egressCmd represents the egress command
var egressCmd = &cobra.Command{
	Use:   "egress",
	Short: "Manage egress rules and traffic",
	Long: `Manage egress rules and outbound traffic configuration.

This command allows you to configure and monitor outbound
network traffic and egress policies for your resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// egressAllowCmd represents the egress allow command
var egressAllowCmd = &cobra.Command{
	Use:   "allow DOMAIN",
	Short: "Allow egress traffic to a domain",
	Long: `Allow outbound traffic to a specific domain via HTTPS.

Examples:
  irons egress allow crates.io
  irons egress allow my-private-registry.com
  irons egress allow api.example.com`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly one DOMAIN argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Allowing egress to '%s'...\n", domain)

		// Make API call
		if err := client.EgressAllow(domain); err != nil {
			return fmt.Errorf("allowing egress: %w", err)
		}

		// Show success
		fmt.Printf("✓ Egress rule added successfully!\n")
		return nil
	},
}

// egressDenyCmd represents the egress deny command
var egressDenyCmd = &cobra.Command{
	Use:   "deny DOMAIN",
	Short: "Deny egress traffic to a domain",
	Long: `Deny outbound traffic to a specific domain.

Examples:
  irons egress deny registry.npmjs.org
  irons egress deny malicious-site.com
  irons egress deny ads.example.com`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly one DOMAIN argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		domain := args[0]

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Denying egress to '%s'...\n", domain)

		// Make API call
		if err := client.EgressDeny(domain); err != nil {
			return fmt.Errorf("denying egress: %w", err)
		}

		// Show success
		fmt.Printf("✓ Egress rule added successfully!\n")
		return nil
	},
}

// egressModeCmd represents the egress mode command
var egressModeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Get or set the egress mode",
	Long: `Get the current egress mode, or set it to enforce or warn.

Examples:
  irons egress mode
  irons egress mode deny
  irons egress mode warn`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		resp, err := client.EgressGetMode()
		if err != nil {
			return fmt.Errorf("getting egress mode: %w", err)
		}

		fmt.Printf("Egress mode: %s\n", resp.Mode)
		return nil
	},
}

// egressModeDenyCmd sets the egress mode to deny
var egressModeDenyCmd = &cobra.Command{
	Use:   "deny",
	Short: "Set egress mode to deny",
	Long:  `Set the egress mode to deny. Egress traffic not matching allow rules will be blocked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		if err := client.EgressSetMode("deny"); err != nil {
			return fmt.Errorf("setting egress mode: %w", err)
		}

		fmt.Printf("✓ Egress mode set to deny\n")
		return nil
	},
}

// egressModeWarnCmd sets the egress mode to warn
var egressModeWarnCmd = &cobra.Command{
	Use:   "warn",
	Short: "Set egress mode to warn",
	Long:  `Set the egress mode to warn. Egress traffic not matching allow rules will be logged but not blocked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		if err := client.EgressSetMode("warn"); err != nil {
			return fmt.Errorf("setting egress mode: %w", err)
		}

		fmt.Printf("✓ Egress mode set to warn\n")
		return nil
	},
}

// egressListCmd represents the egress list command
var egressListCmd = &cobra.Command{
	Use:   "list",
	Short: "List egress rules for the account",
	Long: `List all current egress rules for the account.

Examples:
  irons egress list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Listing egress rules...\n")

		// Make API call
		resp, err := client.EgressList()
		if err != nil {
			return fmt.Errorf("listing egress rules: %w", err)
		}

		// Show results
		fmt.Printf("\n✓ Egress rules:\n")

		if len(resp.AllowedDomains) > 0 {
			fmt.Printf("  Allowed domains:\n")
			for _, domain := range resp.AllowedDomains {
				fmt.Printf("    - %s\n", domain)
			}
		} else {
			fmt.Printf("  Allowed domains: none\n")
		}

		if len(resp.DeniedDomains) > 0 {
			fmt.Printf("  Denied domains:\n")
			for _, domain := range resp.DeniedDomains {
				fmt.Printf("    - %s\n", domain)
			}
		} else {
			fmt.Printf("  Denied domains: none\n")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(egressCmd)

	// Add subcommands
	egressCmd.AddCommand(egressAllowCmd)
	egressCmd.AddCommand(egressDenyCmd)
	egressCmd.AddCommand(egressListCmd)
	egressCmd.AddCommand(egressModeCmd)
	egressModeCmd.AddCommand(egressModeDenyCmd)
	egressModeCmd.AddCommand(egressModeWarnCmd)

}
