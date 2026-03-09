package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ironsh/irons/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const snapshotPollInterval = 3 * time.Second

// snapshotsCmd is the parent command for snapshot operations.
var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "Manage snapshots",
	Long: `Manage VM snapshots.

Snapshots capture a VM's overlay at a point in time with zero downtime.
Restoring a snapshot creates a new VM — the original is untouched.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// snapshotsCreateCmd creates a new snapshot.
var snapshotsCreateCmd = &cobra.Command{
	Use:   "create <vm-id-or-name>",
	Short: "Create a snapshot of a VM",
	Long: `Create a snapshot of the specified VM.

The snapshot is taken with zero downtime. The VM is not affected
if the upload fails.

By default the command returns immediately after the API accepts the
request. Use --wait to poll until the snapshot reaches "ready" or "failed".

Examples:
  irons snapshots create my-dev-env
  irons snapshots create my-dev-env --label pre-refactor
  irons snapshots create vm_k3mf9xvw2p --wait
  irons snapshots create my-dev-env --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		label, _ := cmd.Flags().GetString("label")
		wait, _ := cmd.Flags().GetBool("wait")
		jsonOut, _ := cmd.Flags().GetBool("json")

		client := newClient()

		vmID, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		req := api.CreateSnapshotRequest{Label: label}
		snap, err := client.SnapshotsCreate(vmID, req)
		if err != nil {
			return fmt.Errorf("creating snapshot: %w", err)
		}

		if jsonOut {
			return printJSON(snap)
		}

		if !wait {
			snapLabel := snap.Label
			if snapLabel == "" {
				snapLabel = "(unlabeled)"
			}
			fmt.Printf("%s  %s  %s  %s\n", snap.ID, snapLabel, snap.Status, snap.SourceVirtualMachineID)
			fmt.Printf("Snapshot started. Run 'irons snapshots get %s' to check status.\n", snap.ID)
			return nil
		}

		// Poll until ready or failed.
		if err := waitForSnapshot(cmd.Context(), client, snap.ID); err != nil {
			return err
		}

		// Re-fetch to get final state.
		snap, err = client.SnapshotsGet(snap.ID)
		if err != nil {
			return fmt.Errorf("getting snapshot: %w", err)
		}

		snapLabel := snap.Label
		if snapLabel != "" {
			snapLabel = " (" + snapLabel + ")"
		}
		fmt.Printf("✓ Snapshot ready: %s%s\n", snap.ID, snapLabel)
		return nil
	},
}

// snapshotsListCmd lists snapshots.
var snapshotsListCmd = &cobra.Command{
	Use:   "list [<vm-id-or-name>]",
	Short: "List snapshots",
	Long: `List snapshots, optionally filtered by VM.

Without a VM argument, lists all snapshots across VMs. With a VM argument,
lists only snapshots for that VM.

Examples:
  irons snapshots list
  irons snapshots list my-dev-env
  irons snapshots list vm_k3mf9xvw2p --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		client := newClient()

		var resp *api.ListSnapshotsResponse
		var err error

		if len(args) == 1 {
			vmID, resolveErr := resolveVM(client, args[0])
			if resolveErr != nil {
				return resolveErr
			}
			resp, err = client.SnapshotsListByVM(vmID)
		} else {
			resp, err = client.SnapshotsList()
		}
		if err != nil {
			return fmt.Errorf("listing snapshots: %w", err)
		}

		if jsonOut {
			return printJSON(resp)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No snapshots found.")
			return nil
		}

		// Show VM column only when listing all snapshots (no VM filter).
		showVM := len(args) == 0

		table := tablewriter.NewTable(os.Stdout)
		if showVM {
			table.Header([]string{"ID", "Label", "Status", "VM", "Created"})
			for _, s := range resp.Data {
				table.Append([]string{s.ID, snapshotDisplayLabel(s.Label), s.Status, s.SourceVirtualMachineID, formatRelativeTime(s.CreatedAt)})
			}
		} else {
			table.Header([]string{"ID", "Label", "Status", "Created"})
			for _, s := range resp.Data {
				table.Append([]string{s.ID, snapshotDisplayLabel(s.Label), s.Status, formatRelativeTime(s.CreatedAt)})
			}
		}
		table.Render()

		return nil
	},
}

// snapshotsGetCmd shows details of a single snapshot.
var snapshotsGetCmd = &cobra.Command{
	Use:   "get <snapshot-id>",
	Short: "Show snapshot details",
	Long: `Show detailed information about a snapshot.

Examples:
  irons snapshots get snap_x9f2km4p
  irons snapshots get snap_x9f2km4p --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		jsonOut, _ := cmd.Flags().GetBool("json")

		client := newClient()

		snap, err := client.SnapshotsGet(id)
		if err != nil {
			return fmt.Errorf("getting snapshot: %w", err)
		}

		if jsonOut {
			return printJSON(snap)
		}

		fmt.Printf("  ID:         %s\n", snap.ID)
		fmt.Printf("  Label:      %s\n", snapshotDisplayLabel(snap.Label))
		fmt.Printf("  VM:         %s\n", snap.SourceVirtualMachineID)
		fmt.Printf("  Status:     %s\n", snap.Status)
		if snap.BaseImageID != "" {
			fmt.Printf("  Base image: %s\n", snap.BaseImageID)
		}
		fmt.Printf("  Created:    %s\n", formatRelativeTime(snap.CreatedAt))

		return nil
	},
}

// snapshotsDeleteCmd deletes a snapshot.
var snapshotsDeleteCmd = &cobra.Command{
	Use:   "delete <snapshot-id>",
	Short: "Delete a snapshot",
	Long: `Delete a snapshot. This cannot be undone.

Without --force, the command prompts for confirmation.

Examples:
  irons snapshots delete snap_x9f2km4p
  irons snapshots delete snap_x9f2km4p --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		force, _ := cmd.Flags().GetBool("force")

		client := newClient()

		if !force {
			// Fetch snapshot details for the confirmation prompt.
			snap, err := client.SnapshotsGet(id)
			if err != nil {
				return fmt.Errorf("getting snapshot: %w", err)
			}

			label := snapshotDisplayLabel(snap.Label)
			fmt.Printf("Delete snapshot %s (%s)? This cannot be undone. [y/N] ", id, label)

			if !confirmPrompt() {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := client.SnapshotsDelete(id); err != nil {
			return fmt.Errorf("deleting snapshot: %w", err)
		}

		fmt.Printf("✓ Snapshot %s deleted.\n", id)
		return nil
	},
}

// waitForSnapshot polls until the snapshot reaches "ready" or "failed".
func waitForSnapshot(ctx context.Context, client *api.Client, id string) error {
	deadline := time.Now().Add(pollTimeout)

	lastStatus := ""
	printStatus := func(status string) {
		if status != lastStatus {
			if lastStatus != "" {
				fmt.Println()
			}
			fmt.Printf("⠋ Snapshot %s...", status)
			lastStatus = status
		} else {
			fmt.Print(".")
		}
	}

	printStatus("pending")

	ticker := time.NewTicker(snapshotPollInterval)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			fmt.Println()
			return fmt.Errorf("timed out after %s waiting for snapshot '%s'. The machine was not affected", pollTimeout, id)
		}

		snap, err := client.SnapshotsGet(id)
		if err != nil {
			fmt.Print(".")
		} else if snap.Status == "ready" {
			fmt.Println()
			return nil
		} else if snap.Status == "failed" {
			fmt.Println()
			return fmt.Errorf("✗ Snapshot failed. The machine was not affected")
		} else {
			printStatus(snap.Status)
		}

		select {
		case <-ctx.Done():
			fmt.Println()
			return fmt.Errorf("cancelled while waiting for snapshot '%s': %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}

// confirmPrompt reads a single line from stdin and returns true if the user
// typed "y" or "yes" (case-insensitive).
func confirmPrompt() bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return answer == "y" || answer == "yes"
}

// printJSON marshals v as indented JSON and writes it to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// snapshotDisplayLabel returns the snapshot's label, or "(unlabeled)" if empty.
func snapshotDisplayLabel(label string) string {
	if label == "" {
		return "(unlabeled)"
	}
	return label
}

// formatRelativeTime formats an RFC3339 timestamp as a relative duration
// (e.g. "2h ago", "3d ago") when less than 7 days old, or as an absolute
// date otherwise.
func formatRelativeTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.UTC().Format("2006-01-02 15:04:05 UTC")
	}
}

func init() {
	rootCmd.AddCommand(snapshotsCmd)

	snapshotsCmd.AddCommand(snapshotsCreateCmd)
	snapshotsCmd.AddCommand(snapshotsListCmd)
	snapshotsCmd.AddCommand(snapshotsGetCmd)
	snapshotsCmd.AddCommand(snapshotsDeleteCmd)

	// create flags
	snapshotsCreateCmd.Flags().String("label", "", "Optional label for the snapshot")
	snapshotsCreateCmd.Flags().Bool("wait", false, "Wait for the snapshot to be ready")
	snapshotsCreateCmd.Flags().Bool("json", false, "Output raw JSON")

	// list flags
	snapshotsListCmd.Flags().Bool("json", false, "Output raw JSON")

	// get flags
	snapshotsGetCmd.Flags().Bool("json", false, "Output raw JSON")

	// delete flags
	snapshotsDeleteCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
