package cmd

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ironsh/irons/api"
)

const (
	pollInterval = 2 * time.Second
	pollTimeout  = 10 * time.Minute
)

// statusIn returns a condition func that is satisfied when the VM's status
// matches one of the provided values.
func statusIn(statuses ...string) func(*api.VM) bool {
	return func(vm *api.VM) bool {
		return slices.Contains(statuses, vm.Status)
	}
}

// statusAndDetailEq returns a condition func that is satisfied when the VM's
// status matches the provided status and status detail matches the provided detail.
func statusAndDetailEq(status string, detail string) func(*api.VM) bool {
	return func(vm *api.VM) bool {
		return vm.Status == status && vm.StatusDetail == detail
	}
}

// waitForVMCond polls the VM until cond returns true, the timeout is
// exceeded, or ctx is cancelled. It prints progress to stdout.
func waitForVMCond(ctx context.Context, client *api.Client, id string, cond func(*api.VM) bool) error {
	deadline := time.Now().Add(pollTimeout)

	fmt.Printf("Waiting for VM '%s'", id)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			fmt.Println()
			return fmt.Errorf("timed out after %s waiting for VM '%s'", pollTimeout, id)
		}

		resp, err := client.GetVM(id)
		if err != nil {
			// Transient network errors shouldn't abort the wait; just retry.
			fmt.Print(".")
		} else if resp.Status == "failed" {
			fmt.Println()
			return fmt.Errorf("VM '%s' entered failed state", id)
		} else if cond(resp) {
			fmt.Println()
			return nil
		} else {
			fmt.Print(".")
		}

		// Wait for the next poll interval or an early exit signal.
		select {
		case <-ctx.Done():
			fmt.Println()
			return fmt.Errorf("cancelled while waiting for VM '%s': %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}
