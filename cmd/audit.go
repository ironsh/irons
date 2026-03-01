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
	Long:  `View audit logs for VM activity.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var auditEgressCmd = &cobra.Command{
	Use:   "egress",
	Short: "View egress audit logs",
	Long: `View egress audit logs.

Prints a log of outbound network connection attempts, including whether each
was allowed or denied. Use --follow to continuously tail new events.

Examples:
  irons audit egress
  irons audit egress --vm vm_abc123
  irons audit egress --verdict blocked
  irons audit egress --follow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		follow, _ := cmd.Flags().GetBool("follow")
		vmID, _ := cmd.Flags().GetString("vm")
		verdict, _ := cmd.Flags().GetString("verdict")
		since, _ := cmd.Flags().GetString("since")
		until, _ := cmd.Flags().GetString("until")
		limit, _ := cmd.Flags().GetInt("limit")

		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		params := api.AuditEgressParams{
			VMID:    vmID,
			Verdict: verdict,
			Since:   since,
			Until:   until,
			Limit:   limit,
		}

		var cursor string

		printEvents := func(resp *api.ListAuditEgressResponse) {
			for _, ev := range resp.Data {
				printEgressEvent(ev)
			}
			if resp.Cursor != nil {
				cursor = *resp.Cursor
			}
		}

		// Initial fetch.
		resp, err := client.AuditEgress(params)
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
				params.Cursor = cursor
				resp, err := client.AuditEgress(params)
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
	if ev.VMID != "" {
		parts = append(parts, ev.VMID)
	}
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
	auditEgressCmd.Flags().String("vm", "", "Filter by VM ID")
	auditEgressCmd.Flags().String("verdict", "", "Filter by verdict (allowed, blocked, warn)")
	auditEgressCmd.Flags().String("since", "", "Show events after this timestamp (RFC3339)")
	auditEgressCmd.Flags().String("until", "", "Show events before this timestamp (RFC3339)")
	auditEgressCmd.Flags().Int("limit", 0, "Maximum number of events to return")
}
