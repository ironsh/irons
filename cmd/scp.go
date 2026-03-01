package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// scpCmd represents the scp command
var scpCmd = &cobra.Command{
	Use:   "scp SRC DST",
	Short: "Copy files to/from a VM via SCP",
	Long: `Copy files to or from a VM using SCP.

Prefix a path with the VM ID and a colon to denote a remote path, e.g.:

  # Upload a local file to the VM
  irons scp ./local-file.txt vm_abc123:/remote/path/

  # Download a file from the VM
  irons scp vm_abc123:/remote/file.txt ./local-dest/

The VM connection details (host, port, username) are resolved
automatically from the API.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		dst := args[1]

		showCommand, _ := cmd.Flags().GetBool("command")
		strictHostKeys, _ := cmd.Flags().GetBool("strict-hostkeys")
		recursive, _ := cmd.Flags().GetBool("recursive")

		// parseVMPath checks whether a path is in "id:remotepath" form.
		// It returns the VM ID and the remote path if so, or ("", "") if not.
		// We avoid misidentifying Windows-style drive letters (e.g. C:\...) by
		// requiring the prefix before the colon to be longer than one character.
		parseVMPath := func(p string) (id, remotePath string, ok bool) {
			idx := strings.Index(p, ":")
			if idx <= 1 {
				return "", "", false
			}
			return p[:idx], p[idx+1:], true
		}

		// Exactly one of src or dst must be a VM path.
		srcID, srcRemote, srcIsRemote := parseVMPath(src)
		dstID, dstRemote, dstIsRemote := parseVMPath(dst)

		if srcIsRemote && dstIsRemote {
			return fmt.Errorf("only one of SRC or DST may be a VM path (id:path)")
		}
		if !srcIsRemote && !dstIsRemote {
			return fmt.Errorf("one of SRC or DST must be a VM path (id:path)")
		}

		var idOrName string
		if srcIsRemote {
			idOrName = srcID
		} else {
			idOrName = dstID
		}

		// Create API client
		client := newClient()

		id, err := resolveVM(client, idOrName)
		if err != nil {
			return err
		}

		// Resolve SSH connection info (scp connects over SSH)
		fmt.Printf("Getting SSH connection info for VM '%s'...\n", id)

		resp, err := client.SSH(id)
		if err != nil {
			return fmt.Errorf("getting SSH info: %w", err)
		}

		remote := fmt.Sprintf("%s@%s", resp.Username, resp.Host)

		// Replace the id:path form with user@host:path.
		if srcIsRemote {
			src = fmt.Sprintf("%s:%s", remote, srcRemote)
		}
		if dstIsRemote {
			dst = fmt.Sprintf("%s:%s", remote, dstRemote)
		}

		// Build scp argument list
		scpArgs := []string{
			"-P", fmt.Sprintf("%d", resp.Port),
		}

		if recursive {
			scpArgs = append(scpArgs, "-r")
		}

		if !strictHostKeys {
			scpArgs = append(scpArgs,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "LogLevel=ERROR",
			)
		}

		scpArgs = append(scpArgs, src, dst)

		// If --command flag is set, just print the command and exit
		if showCommand {
			fmt.Print("scp")
			for _, arg := range scpArgs {
				fmt.Printf(" %s", arg)
			}
			fmt.Println()
			return nil
		}

		// Execute scp
		fmt.Printf("Copying %s -> %s ...\n", src, dst)

		scpExec := exec.Command("scp", scpArgs...)
		scpExec.Stdin = os.Stdin
		scpExec.Stdout = os.Stdout
		scpExec.Stderr = os.Stderr

		if err := scpExec.Run(); err != nil {
			return fmt.Errorf("scp command failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(scpCmd)

	scpCmd.Flags().BoolP("command", "c", false, "Output SCP command instead of executing it")
	scpCmd.Flags().Bool("strict-hostkeys", false, "Enable strict host key checking (disabled by default)")
	scpCmd.Flags().BoolP("recursive", "r", false, "Recursively copy entire directories")
}
