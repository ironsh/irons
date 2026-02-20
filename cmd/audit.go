package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	Long:  `View audit logs for sandbox activity.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var auditEgressCmd = &cobra.Command{
	Use:   "egress NAME",
	Short: "View egress audit logs for a sandbox",
	Long: `View egress audit logs for a sandbox.

Prints a log of outbound network connection attempts, including whether each
was allowed or denied. Use --follow to continuously tail new events.

Examples:
  irons audit egress my-sandbox
  irons audit egress my-sandbox --follow`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires exactly one NAME argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		follow, _ := cmd.Flags().GetBool("follow")

		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		var pageToken int64

		printEvents := func(resp *api.EgressAuditResponse) {
			for _, ev := range resp.Events {
				printEgressEvent(ev)
			}
			if resp.PageToken != 0 {
				pageToken = resp.PageToken
			}
		}

		// Initial fetch.
		resp, err := client.AuditEgress(name, pageToken)
		if err != nil {
			return fmt.Errorf("fetching egress audit log: %w", err)
		}
		printEvents(resp)

		if !follow {
			return nil
		}

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				resp, err := client.AuditEgress(name, pageToken)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
					continue
				}
				printEvents(resp)
			}
		}
	},
}

func printEgressEvent(ev api.EgressAuditEvent) {
	verdict := "DENIED"
	if ev.Allowed {
		verdict = "ALLOWED"
	}

	ts := ev.Timestamp.Local().Format(time.RFC3339)

	if ev.Mode != "" {
		fmt.Printf("%s  %-7s  %s  (mode: %s)\n", ts, verdict, ev.Host, ev.Mode)
	} else {
		fmt.Printf("%s  %-7s  %s\n", ts, verdict, ev.Host)
	}
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditEgressCmd)

	auditEgressCmd.Flags().BoolP("follow", "f", false, "Continuously poll for new events (like tail -f)")
}
