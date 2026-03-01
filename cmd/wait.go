package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ironsh/irons/api"
)

const (
	pollInterval = 2 * time.Second
	pollTimeout  = 10 * time.Minute
)

// waitForStatus polls the VM status until it matches one of the expected
// statuses, until the timeout is exceeded, or until ctx is cancelled. It
// prints progress to stdout.
func waitForStatus(ctx context.Context, client *api.Client, id string, expected []string) error {
	deadline := time.Now().Add(pollTimeout)

	fmt.Printf("Waiting for VM '%s' to be %s", id, strings.Join(expected, " or "))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			fmt.Println()
			return fmt.Errorf("timed out after %s waiting for VM '%s' to be %s",
				pollTimeout, id, strings.Join(expected, " or "))
		}

		resp, err := client.GetVM(id)
		if err != nil {
			// Transient network errors shouldn't abort the wait; just retry.
			fmt.Print(".")
		} else if resp.Status == "failed" {
			fmt.Println()
			return fmt.Errorf("VM '%s' entered failed state", id)
		} else if slices.Contains(expected, resp.Status) {
			fmt.Println()
			return nil
		} else {
			fmt.Print(".")
		}

		// Wait for the next poll interval or an early exit signal.
		select {
		case <-ctx.Done():
			fmt.Println()
			return fmt.Errorf("cancelled while waiting for VM '%s' to be %s: %w",
				id, strings.Join(expected, " or "), ctx.Err())
		case <-ticker.C:
		}
	}
}
