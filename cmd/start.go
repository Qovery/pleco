package cmd

import (
	"github.com/Qovery/pleco/pleco"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Pleco as a daemon",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()
		pleco.StartDaemon()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("dry-run", "t", false, "Dry run mode")
}