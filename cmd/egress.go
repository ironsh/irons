package cmd

import (
	"fmt"
	"os"

	"github.com/ironsh/irons/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
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

// egressAddCmd creates a new egress rule
var egressAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an egress rule",
	Long: `Add an egress rule to allow outbound traffic to a host or CIDR.

Examples:
  irons egress add --host crates.io
  irons egress add --cidr 10.0.0.0/8 --name "internal network" --comment "Allow internal traffic"
  irons egress add --host api.example.com --name "example api"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		cidr, _ := cmd.Flags().GetString("cidr")
		name, _ := cmd.Flags().GetString("name")
		comment, _ := cmd.Flags().GetString("comment")

		if host == "" && cidr == "" {
			return fmt.Errorf("either --host or --cidr is required")
		}
		if host != "" && cidr != "" {
			return fmt.Errorf("only one of --host or --cidr may be specified")
		}

		client := newClient()

		req := api.EgressRuleRequest{
			Name:    name,
			Host:    host,
			CIDR:    cidr,
			Comment: comment,
		}

		rule, err := client.EgressCreateRule(req)
		if err != nil {
			return fmt.Errorf("creating egress rule: %w", err)
		}

		fmt.Printf("✓ Egress rule created (ID: %s)\n", rule.ID)
		return nil
	},
}

// egressRemoveCmd removes an egress rule by ID
var egressRemoveCmd = &cobra.Command{
	Use:   "remove RULE_ID",
	Short: "Remove an egress rule",
	Long: `Remove an egress rule by its ID.

Examples:
  irons egress remove rule_abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleID := args[0]

		client := newClient()

		if err := client.EgressDeleteRule(ruleID); err != nil {
			return fmt.Errorf("removing egress rule: %w", err)
		}

		fmt.Printf("✓ Egress rule '%s' removed.\n", ruleID)
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
		client := newClient()

		table := tablewriter.NewTable(os.Stdout)
		table.Header([]string{"ID", "Name", "Host/CIDR", "Comment"})

		var total int
		err := paginate(func(cursor string) (string, bool, error) {
			resp, err := client.EgressListRules(cursor)
			if err != nil {
				return "", false, fmt.Errorf("listing egress rules: %w", err)
			}
			for _, r := range resp.Data {
				target := r.Host
				if target == "" {
					target = r.CIDR
				}
				table.Append([]string{r.ID, r.Name, target, r.Comment})
			}
			total += len(resp.Data)
			var next string
			if resp.Cursor != nil {
				next = *resp.Cursor
			}
			return next, resp.HasMore, nil
		})
		if err != nil {
			return err
		}

		if total == 0 {
			fmt.Println("No egress rules found.")
			return nil
		}

		table.Render()
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
  irons egress mode enforce
  irons egress mode warn`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		resp, err := client.EgressGetPolicy()
		if err != nil {
			return fmt.Errorf("getting egress mode: %w", err)
		}

		fmt.Printf("Egress mode: %s\n", resp.Mode)
		return nil
	},
}

// egressModeEnforceCmd sets the egress mode to enforce
var egressModeEnforceCmd = &cobra.Command{
	Use:   "enforce",
	Short: "Set egress mode to enforce",
	Long:  `Set the egress mode to enforce. Egress traffic not matching allow rules will be blocked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		if err := client.EgressSetPolicy("enforce"); err != nil {
			return fmt.Errorf("setting egress mode: %w", err)
		}

		fmt.Printf("✓ Egress mode set to enforce\n")
		return nil
	},
}

// egressModeWarnCmd sets the egress mode to warn
var egressModeWarnCmd = &cobra.Command{
	Use:   "warn",
	Short: "Set egress mode to warn",
	Long:  `Set the egress mode to warn. Egress traffic not matching allow rules will be logged but not blocked.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newClient()

		if err := client.EgressSetPolicy("warn"); err != nil {
			return fmt.Errorf("setting egress mode: %w", err)
		}

		fmt.Printf("✓ Egress mode set to warn\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(egressCmd)

	// Add subcommands
	egressCmd.AddCommand(egressAddCmd)
	egressCmd.AddCommand(egressRemoveCmd)
	egressCmd.AddCommand(egressListCmd)
	egressCmd.AddCommand(egressModeCmd)
	egressModeCmd.AddCommand(egressModeEnforceCmd)
	egressModeCmd.AddCommand(egressModeWarnCmd)

	// Flags for add command
	egressAddCmd.Flags().String("host", "", "Host to allow egress to")
	egressAddCmd.Flags().String("cidr", "", "CIDR range to allow egress to")
	egressAddCmd.Flags().String("name", "", "Optional name for the rule")
	egressAddCmd.Flags().String("comment", "", "Optional comment for the rule")
}
