package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ironsh/irons/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DefaultAPIURL = "https://api.iron.sh/v1"

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

		if viper.GetString("api-key") == "" {
			requireAuth()
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
	rootCmd.PersistentFlags().Bool("debug-api", false, "Dump API requests and responses to stderr")
	rootCmd.PersistentFlags().Bool("debug-tls-skip-verify", false, "Disable TLS certificate verification (insecure, for debugging only)")

	// Bind flags to environment variables
	viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("api-key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("debug-api", rootCmd.PersistentFlags().Lookup("debug-api"))
	viper.BindPFlag("debug-tls-skip-verify", rootCmd.PersistentFlags().Lookup("debug-tls-skip-verify"))

	// Set environment variable names
	viper.BindEnv("api-url", "IRONS_API_URL")
	viper.BindEnv("api-key", "IRONS_API_KEY")
	viper.BindEnv("debug-api", "IRONS_DEBUG_API")
	viper.BindEnv("debug-tls-skip-verify", "IRONS_DEBUG_TLS_SKIP_VERIFY")

	// Load the API key from ~/.config/irons/config.yml (written by `irons login`).
	// A flag or environment variable always takes precedence over the config file.
	if viper.GetString("api-key") == "" {
		if cfg, err := config.Load(); err == nil && cfg.APIKey != "" {
			viper.Set("api-key", cfg.APIKey)
		}
	}

	// Cobra also supports local flags which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
