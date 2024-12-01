package cmd

import (
	"hsf/src/logging"
	"hsf/src/website"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Short: "Runs the hsf server",
	Run: func(cmd *cobra.Command, args []string) {
		website.Start()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logging.Error().Err(err).Msg("Cobra execution failed")
		os.Exit(1)
	}
}
