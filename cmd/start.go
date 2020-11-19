package cmd

import (
	"fmt"
	"github.com/Qovery/pleco/core"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Pleco as a daemon",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()

		disableDryRun, _ := cmd.Flags().GetBool("disable-dry-run")
		interval, _ := cmd.Flags().GetInt64("check-interval")

		fmt.Println("\n\n ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery\n\n")
		log.Info("Starting Pleco")

		core.StartDaemon(disableDryRun, interval, cmd)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("disable-dry-run", "y", false, "Disable dry run mode")
	startCmd.Flags().Int64P("check-interval", "i", 120, "Check interval in seconds")

	// AWS - Databases
	startCmd.Flags().BoolP("enable-rds", "r", true, "Enable RDS watch")
	startCmd.Flags().BoolP("enable-documentdb", "m", true, "Enable DocumentDB watch")
	startCmd.Flags().BoolP("enable-elasticache", "c", true, "Enable Elasticache watch")

	// K8s
	startCmd.Flags().StringP("kube-conn", "k", "off","Kubernetes connection method, choose between : off/in/out")
}