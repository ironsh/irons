package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const DefaultAPIURL = "https://elrond.ironcd.dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "irons",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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

func Execute() {
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
