package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironcd/irons/api"
)

const (
	pollInterval = 2 * time.Second
	pollTimeout  = 10 * time.Minute
)

// waitForStatus polls the sandbox status until it matches one of the expected
// statuses, or until the timeout is exceeded. It prints progress to stdout.
func waitForStatus(client *api.Client, name string, expected []string) error {
	deadline := time.Now().Add(pollTimeout)

	fmt.Printf("Waiting for sandbox '%s' to be %s", name, strings.Join(expected, " or "))

	for time.Now().Before(deadline) {
		resp, err := client.Status(name)
		if err != nil {
			// Transient network errors shouldn't abort the wait; just retry.
			fmt.Print(".")
			time.Sleep(pollInterval)
			continue
		}

		current := strings.ToLower(resp.Status)

		// Check whether we've reached a terminal error state.
		if strings.Contains(current, "failed") {
			fmt.Println()
			return fmt.Errorf("sandbox '%s' entered error state: %s", name, resp.Status)
		}

		// Check whether we've reached the desired state.
		for _, want := range expected {
			if strings.Contains(current, strings.ToLower(want)) {
				fmt.Println()
				return nil
			}
		}

		fmt.Print(".")
		time.Sleep(pollInterval)
	}

	fmt.Println()
	return fmt.Errorf("timed out after %s waiting for sandbox '%s' to be %s",
		pollTimeout, name, strings.Join(expected, " or "))
}
