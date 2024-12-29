package cmd

import (
	"hsf/src/utils"
	"hsf/src/website"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Short: "Runs the hsf server",
	Run: func(cmd *cobra.Command, args []string) {
		website.Start()
	},
}

func Execute() {
	utils.Must(rootCmd.Execute())
}
