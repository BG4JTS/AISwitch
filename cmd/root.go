package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ais",
	Short: "AI Switch - AI API proxy and format converter",
	Long:  `AI Switch is a proxy server that converts between different AI API formats.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
}