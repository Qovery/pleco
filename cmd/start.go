package cmd

import (
	"github.com/Qovery/pleco/pleco"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Pleco as a daemon",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			log.Info("")
		}
		pleco.StartDaemon(dryRun)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("dry-run", "t", false, "Dry run mode")
}