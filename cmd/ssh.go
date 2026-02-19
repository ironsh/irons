package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ironcd/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh NAME",
	Short: "SSH into a sandbox",
	Long: `SSH into a sandbox to execute commands or open an interactive session.

This command allows you to connect to a specific sandbox via SSH
with the specified configuration and credentials.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		showCommand, _ := cmd.Flags().GetBool("command")
		strictHostKeys, _ := cmd.Flags().GetBool("strict-hostkeys")

		// Create API client
		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		// Get SSH connection info
		fmt.Printf("Getting SSH connection info for sandbox '%s'...\n", name)

		resp, err := client.SSH(name)
		if err != nil {
			return fmt.Errorf("getting SSH info: %w", err)
		}

		// Build SSH command
		sshArgs := []string{
			"-p", fmt.Sprintf("%d", resp.Port),
		}

		if !strictHostKeys {
			sshArgs = append(sshArgs,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
			)
		}

		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", resp.Username, resp.Host))

		// If there's a specific command, add it
		if resp.Command != "" {
			sshArgs = append(sshArgs, resp.Command)
		}

		// If --command flag is set, just output the command
		if showCommand {
			fmt.Printf("ssh")
			for _, arg := range sshArgs {
				fmt.Printf(" %s", arg)
			}
			fmt.Println()
			return nil
		}

		// Execute SSH command
		fmt.Printf("Connecting to %s@%s:%d...\n", resp.Username, resp.Host, resp.Port)

		sshCmd := exec.Command("ssh", sshArgs...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr

		err = sshCmd.Run()
		if err != nil {
			return fmt.Errorf("SSH command failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)

	// Define flags
	sshCmd.Flags().BoolP("command", "c", false, "Output SSH command instead of executing it")
	sshCmd.Flags().Bool("strict-hostkeys", false, "Enable strict host key checking (disabled by default)")
}
