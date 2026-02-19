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
		name, _ := cmd.Flags().GetString("name")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Allowing egress to '%s' for sandbox '%s'...\n", domain, name)

		// Make API call
		resp, err := client.EgressAllow(name, domain)
		if err != nil {
			return fmt.Errorf("allowing egress: %w", err)
		}

		// Show success
		fmt.Printf("✓ Egress rule added successfully!\n")
		fmt.Printf("  Domain: %s\n", resp.Domain)
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
		name, _ := cmd.Flags().GetString("name")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Denying egress to '%s' for sandbox '%s'...\n", domain, name)

		// Make API call
		resp, err := client.EgressDeny(name, domain)
		if err != nil {
			return fmt.Errorf("denying egress: %w", err)
		}

		// Show success
		fmt.Printf("✓ Egress rule added successfully!\n")
		fmt.Printf("  Domain: %s\n", resp.Domain)
		return nil
	},
}

// egressListCmd represents the egress list command
var egressListCmd = &cobra.Command{
	Use:   "list",
	Short: "List egress rules for a sandbox",
	Long: `List all current egress rules for a specific sandbox.

Examples:
  irons egress list --name my-sandbox
  irons egress list -n prod-sandbox`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're doing
		fmt.Printf("Listing egress rules for sandbox '%s'...\n", name)

		// Make API call
		resp, err := client.EgressList(name)
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

	// Individual flags for each subcommand
	egressAllowCmd.Flags().StringP("name", "n", "", "Name of sandbox (default: most recent)")
	egressDenyCmd.Flags().StringP("name", "n", "", "Name of sandbox (default: most recent)")
	egressListCmd.Flags().StringP("name", "n", "", "Name of sandbox (default: most recent)")

	// Mark name as required for subcommands
	egressAllowCmd.MarkFlagRequired("name")
	egressDenyCmd.MarkFlagRequired("name")
	egressListCmd.MarkFlagRequired("name")
}
