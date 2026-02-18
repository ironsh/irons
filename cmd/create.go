/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sandbox",
	Long: `Create a new sandbox with the specified configuration.

This command allows you to create a new sandbox with SSH key,
secrets, and custom naming options.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyPath, _ := cmd.Flags().GetString("key")
		secretStrings, _ := cmd.Flags().GetStringSlice("secret")
		name, _ := cmd.Flags().GetString("name")

		// Read SSH key file
		keyContent, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("reading SSH key file %s: %w", keyPath, err)
		}

		// Parse secrets into map
		secrets := make(map[string]string)
		for _, secret := range secretStrings {
			parts := strings.SplitN(secret, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid secret format '%s'. Expected KEY=VALUE", secret)
			}
			secrets[parts[0]] = parts[1]
		}

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Show what we're creating
		fmt.Printf("Creating sandbox '%s'...\n", name)

		// Make API call
		resp, err := client.Create(keyContent, secrets, name)
		if err != nil {
			return fmt.Errorf("creating sandbox: %w", err)
		}

		// Show success
		fmt.Printf("✓ Sandbox created successfully!\n")
		fmt.Printf("  ID: %s\n", resp.ID)
		fmt.Printf("  Name: %s\n", resp.Name)
		fmt.Printf("  Status: %s\n", resp.Status)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Get default SSH key path with error handling
	defaultKeyPath := "" // fallback default
	if homeDir, err := os.UserHomeDir(); err == nil {
		defaultKeyPath = filepath.Join(homeDir, ".ssh", "id_rsa.pub")
	}

	// Define flags
	createCmd.Flags().StringP("key", "k", defaultKeyPath, "SSH public key path")
	createCmd.Flags().StringSliceP("secret", "s", []string{}, "Inject secret as KEY=VALUE (repeatable)")
	createCmd.Flags().StringP("name", "n", "", "Name of the sandbox")

	// Mark required flags
	createCmd.MarkFlagRequired("name")
}
