package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/ironsh/irons/api"
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

var (
	verdictAllow = color.New(color.FgGreen, color.Bold).SprintfFunc()
	verdictWarn  = color.New(color.FgYellow, color.Bold).SprintfFunc()
	verdictDeny  = color.New(color.FgRed, color.Bold).SprintfFunc()
)

func printEgressEvent(ev api.EgressAuditEvent) {
	verdict := strings.ToLower(ev.Verdict)
	if verdict == "" {
		if ev.Allowed {
			verdict = "allowed"
		} else {
			verdict = "blocked"
		}
	}

	var label string
	switch verdict {
	case "allowed":
		label = verdictAllow("%-5s", "ALLOW")
	case "warn":
		label = verdictWarn("%-5s", "WARN")
	default:
		label = verdictDeny("%-5s", "DENY")
	}

	ts := ev.Timestamp.Local().Format(time.RFC3339)

	var parts []string
	parts = append(parts, ts)
	parts = append(parts, label)
	if ev.Protocol != "" {
		parts = append(parts, fmt.Sprintf("%-5s", ev.Protocol))
	}
	parts = append(parts, ev.Host)
	if ev.Mode != "" {
		parts = append(parts, fmt.Sprintf("(mode: %s)", ev.Mode))
	}

	fmt.Println(strings.Join(parts, "  "))
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditEgressCmd)

	auditEgressCmd.Flags().BoolP("follow", "f", false, "Continuously poll for new events (like tail -f)")
}
