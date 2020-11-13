package cmd

import (
	"github.com/Qovery/pleco/core"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Pleco as a daemon",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		interval, _ := cmd.Flags().GetInt64("check-interval")
		core.StartDaemon(dryRun, interval)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("dry-run", "t", false, "Dry run mode")
	startCmd.Flags().Int64P("check-interval", "i", 120, "Check interval in seconds")
}