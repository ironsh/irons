package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DefaultAPIURL = "https://elrond.ironcd.dev"

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
		// Skip validation for commands that don't need an API key.
		if cmd.Name() == "help" || cmd.Name() == "login" || (cmd.Name() == "irons" && len(args) == 0) {
			return
		}

		apiKey := viper.GetString("api-key")
		if apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: API key is required. Run `irons login`, set --api-key, or set IRONS_API_KEY.\n")
			os.Exit(1)
		}
	},
}

func Execute(version string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd.Version = version
	err := rootCmd.ExecuteContext(ctx)
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

	// Load config file from ~/.config/irons/config.yml (or $XDG_CONFIG_HOME).
	// The key in the YAML file is "api_key", which we map to viper's "api-key".
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		}
	}
	if base != "" {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(filepath.Join(base, "irons"))
		// Ignore "file not found" errors — the config is optional.
		_ = viper.ReadInConfig()

		// Bridge the yaml key "api_key" into viper's "api-key" so that
		// flag/env/file all resolve through the same viper.GetString call.
		if viper.GetString("api-key") == "" {
			if saved := viper.GetString("api_key"); saved != "" {
				viper.Set("api-key", saved)
			}
		}
	}

	// Cobra also supports local flags which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
