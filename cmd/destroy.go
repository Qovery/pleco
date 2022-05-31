package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Qovery/pleco/pkg"
	"github.com/Qovery/pleco/pkg/common"
)

var destroy = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy resources having a specific tag value",
	Run: func(cmd *cobra.Command, args []string) {
		_ = setLogLevel()

		fmt.Println("")
		fmt.Println(" ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery")
		log.Infof("Starting Pleco %s", GetCurrentVersion())

		if commandIsValid(cmd, args) {
			disableDryRun, _ := cmd.Flags().GetBool("disable-dry-run")
			pkg.StartDestroy(args[0], disableDryRun, cmd)
		}
	},
}

func init() {
	rootCmd.AddCommand(destroy)

	destroy.Flags().StringP("tag-name", "", "", "The tag name attached to a resource to clean")
	destroy.Flags().StringP("tag-value", "", "", "The corresponding tag value attached to a resource to clean")
	destroy.Flags().BoolP("disable-dry-run", "y", false, "Disable dry run mode")
	destroy.Flags().StringP("kube-conn", "k", "off", "Kubernetes connection method, choose between : off/in/out")

	if len(os.Args) > 2 {
		common.InitFlags(os.Args[2], destroy)
	}
}

func commandIsValid(cmd *cobra.Command, args []string) bool {
	valid := true

	if len(args) < 1 {
		log.Errorf("The cloud provider is mandatory (aws, scaleway, do)")
	}

	tagName, err := cmd.Flags().GetString("tag-name")
	if err != nil || strings.TrimSpace(tagName) == "" {
		log.Errorf("The 'tag-name' option is mandatory")
		valid = false
	}

	tagValue, err := cmd.Flags().GetString("tag-value")
	if err != nil || strings.TrimSpace(tagValue) == "" {
		log.Errorf("The 'tag-value' option is mandatory")
		valid = false
	}
	return valid
}
