package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DefaultAPIURL = "https://elrond.ironsh.dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "irons",
	Short: "Spin up egress-secured cloud VMs for AI agents",
	Long: `irons is a CLI tool for spinning up egress-secured cloud VMs (sandboxes) designed for use with AI agents.

It lets you create isolated, SSH-accessible environments with fine-grained control over outbound network
traffic — so you can give an agent a real machine to work in without giving it unfettered internet access.

Each sandbox is a cloud VM provisioned through the IronCD API. Egress rules are enforced at the network
level, meaning you can allowlist only the domains an agent needs to reach (e.g. a package registry, an
internal API) and block everything else. Rules can also be set to warn mode, which logs violations without
blocking them — useful for auditing before locking things down.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip validation for help command and root command without subcommands
		if cmd.Name() == "help" || cmd.Name() == "irons" && len(args) == 0 {
			return
		}

		apiKey := viper.GetString("api-key")
		if apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: API key is required. Set it with --api-key flag or IRONS_API_KEY environment variable.\n")
			os.Exit(1)
		}
	},
}

func Execute(version string) {
	rootCmd.Version = version
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().String("api-url", DefaultAPIURL, "API endpoint URL")
	rootCmd.PersistentFlags().String("api-key", "", "API key for authentication")

	// Bind flags to environment variables
	viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("api-key", rootCmd.PersistentFlags().Lookup("api-key"))

	// Set environment variable names
	viper.BindEnv("api-url", "IRONS_API_URL")
	viper.BindEnv("api-key", "IRONS_API_KEY")

	// Cobra also supports local flags which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
