package cmd

import (
	"fmt"

	"github.com/ironsh/irons/api"
	"github.com/spf13/viper"
)

// newClient builds an api.Client from the current viper configuration.
// It reads api-url, api-key, and debug-api so callers don't have to.
func newClient() *api.Client {
	return api.NewClientDebug(
		viper.GetString("api-url"),
		viper.GetString("api-key"),
		viper.GetBool("debug-api"),
	)
}

// resolveVM resolves a VM name or ID to a VM ID using the provided client.
// If idOrName starts with "vm_" it is returned unchanged. Otherwise the list
// VMs endpoint is queried by name and the first non-destroyed VM's ID is
// returned. An error is returned if no matching VM is found.
func resolveVM(client *api.Client, idOrName string) (string, error) {
	id, err := client.ResolveVM(idOrName)
	if err != nil {
		return "", fmt.Errorf("resolving VM %q: %w", idOrName, err)
	}
	return id, nil
}
