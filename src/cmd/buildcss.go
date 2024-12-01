package cmd

import (
	"fmt"
	"hsf/src/buildcss"
	"hsf/src/logging"
	"os"

	"github.com/spf13/cobra"
)

var buildCssCommand = &cobra.Command{
	Use:   "buildcss",
	Short: "Build the website CSS",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, err := buildcss.BuildContext()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		res := ctx.Rebuild()
		outputFilenames := make([]string, 0)
		for _, o := range res.OutputFiles {
			outputFilenames = append(outputFilenames, o.Path)
		}
		logging.Info().
			Interface("Errors", res.Errors).
			Interface("Warnings", res.Warnings).
			Msg("Ran esbuild")
		if len(outputFilenames) > 0 {
			logging.Info().Interface("Files", outputFilenames).Msg("Wrote files")
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCssCommand)
}
