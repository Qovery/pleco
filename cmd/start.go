package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Qovery/pleco/pkg"
	"github.com/Qovery/pleco/pkg/common"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Pleco as a daemon",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()

		disableDryRun, _ := cmd.Flags().GetBool("disable-dry-run")
		interval, _ := cmd.Flags().GetInt64("check-interval")

		fmt.Println("")
		fmt.Println(" ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery")
		log.Infof("Starting Pleco %s", GetCurrentVersion())

		if len(args) < 1 {
			log.Errorf("The provider is mandatory (aws | scaleway | do)")
			os.Exit(1)
		}

		pkg.StartDaemon(args[0], disableDryRun, interval, cmd)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("disable-dry-run", "y", false, "Disable dry run mode")
	startCmd.Flags().Int64P("check-interval", "i", 120, "Check interval in seconds")
	startCmd.Flags().StringP("tag-name", "t", "ttl", "Set the tag name to check for deletion")
	startCmd.Flags().StringP("kube-conn", "k", "off", "Kubernetes connection method, choose between : off/in/out")

	if len(os.Args) > 2 {
		common.InitFlags(os.Args[2], startCmd)
	}
}
