package cmd

import (
	"fmt"
	"os"

	"github.com/ironsh/irons/api"
	"github.com/spf13/viper"
)

// requireAuth prints a descriptive error message when no API key is available
// and exits with a non-zero status code. Call this whenever a command requires
// authentication but none is configured.
func requireAuth() {
	fmt.Fprintf(os.Stderr, `Error: not authenticated.

Run the following command to log in:

  irons login

Alternatively, supply your API key via the --api-key flag or the
IRONS_API_KEY environment variable.
`)
	os.Exit(1)
}

// newClient builds an api.Client from the current viper configuration.
// It reads api-url, api-key, and debug-api so callers don't have to.
func newClient() *api.Client {
	return api.NewClientDebug(
		viper.GetString("api-url"),
		viper.GetString("api-key"),
		viper.GetBool("debug-api"),
	)
}

// paginate drives cursor-based pagination. It calls fetchPage in a loop,
// passing the cursor returned by each page, until hasMore is false. fetchPage
// receives an empty string on the first call.
func paginate(fetchPage func(cursor string) (nextCursor string, hasMore bool, err error)) error {
	var cursor string
	for {
		next, hasMore, err := fetchPage(cursor)
		if err != nil {
			return err
		}
		if !hasMore {
			break
		}
		cursor = next
	}
	return nil
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
