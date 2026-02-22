package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ironsh/irons/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var forwardCmd = &cobra.Command{
	Use:   "forward NAME",
	Short: "Forward a remote port to a local port",
	Long: `Forward a port from a sandbox to your local machine via SSH tunneling.

By default, the local port is the same as the remote port. Use --local-port
to override the local port independently.

Example:
  irons forward my-sandbox --remote-port 3000
  irons forward my-sandbox --remote-port 3000 --local-port 8080`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		remotePort, _ := cmd.Flags().GetInt("remote-port")
		localPort, _ := cmd.Flags().GetInt("local-port")
		strictHostKeys, _ := cmd.Flags().GetBool("strict-hostkeys")
		showCommand, _ := cmd.Flags().GetBool("command")

		if remotePort == 0 {
			return fmt.Errorf("--remote-port is required")
		}

		if localPort == 0 {
			localPort = remotePort
		}

		apiURL := viper.GetString("api-url")
		apiKey := viper.GetString("api-key")
		client := api.NewClient(apiURL, apiKey)

		fmt.Printf("Getting SSH connection info for sandbox '%s'...\n", name)

		resp, err := client.SSH(name)
		if err != nil {
			return fmt.Errorf("getting SSH info: %w", err)
		}

		sshArgs := []string{
			"-p", fmt.Sprintf("%d", resp.Port),
			"-L", fmt.Sprintf("%d:localhost:%d", localPort, remotePort),
			"-N",
		}

		if !strictHostKeys {
			sshArgs = append(sshArgs,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
			)
		}

		sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", resp.Username, resp.Host))

		if showCommand {
			fmt.Printf("ssh")
			for _, arg := range sshArgs {
				fmt.Printf(" %s", arg)
			}
			fmt.Println()
			return nil
		}

		fmt.Printf("Forwarding localhost:%d -> %s:%d (via %s@%s:%d)...\n",
			localPort, resp.Host, remotePort, resp.Username, resp.Host, resp.Port)
		fmt.Println("Press Ctrl+C to stop forwarding.")

		fwdCmd := exec.Command("ssh", sshArgs...)
		fwdCmd.Stdin = os.Stdin
		fwdCmd.Stdout = os.Stdout
		fwdCmd.Stderr = os.Stderr

		if err := fwdCmd.Run(); err != nil {
			return fmt.Errorf("port forward failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(forwardCmd)

	forwardCmd.Flags().IntP("remote-port", "r", 0, "Remote port on the sandbox to forward (required)")
	forwardCmd.Flags().IntP("local-port", "l", 0, "Local port to listen on (defaults to --remote-port)")
	forwardCmd.Flags().Bool("strict-hostkeys", false, "Enable strict host key checking (disabled by default)")
	forwardCmd.Flags().BoolP("command", "c", false, "Output SSH command instead of executing it")
}
