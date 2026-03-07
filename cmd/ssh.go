package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh ID [command...]",
	Short: "SSH into a VM",
	Long: `SSH into a VM to execute commands or open an interactive session.

This command allows you to connect to a specific VM via SSH
with the specified configuration and credentials.

Optionally, pass a command to execute on the remote VM:
  irons ssh myvm ls -la
  irons ssh -t myvm tmux attach`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		showCommand, _ := cmd.Flags().GetBool("command")
		strictHostKeys, _ := cmd.Flags().GetBool("strict-hostkeys")
		forceTTY, _ := cmd.Flags().GetBool("tty")

		// Create API client
		client := newClient()

		id, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		// Get SSH connection info
		fmt.Printf("Getting SSH connection info for VM '%s'...\n", id)

		resp, err := client.SSH(id)
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

		remoteCmd := args[1:]
		if forceTTY {
			sshArgs = append(sshArgs, "-t")
		}

		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", resp.Username, resp.Host))

		// Append remote command as varargs (mimics ssh behavior)
		sshArgs = append(sshArgs, remoteCmd...)

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
	sshCmd.Flags().BoolP("tty", "t", false, "Force pseudo-TTY allocation (useful for interactive commands like tmux)")
}
