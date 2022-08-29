package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetCurrentVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func GetCurrentVersion() string {
	return "0.13.13" // ci-version-check
}
