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

		fmt.Println("")
		fmt.Println(" ____  _     _____ ____ ___  \n|  _ \\| |   | ____/ ___/ _ \\ \n| |_) | |   |  _|| |  | | | |\n|  __/| |___| |__| |__| |_| |\n|_|   |_____|_____\\____\\___/\nBy Qovery")
		fmt.Println("")
		log.Infof("Starting Pleco %s", GetCurrentVersion())

		core.StartDaemon(disableDryRun, interval, cmd)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolP("disable-dry-run", "y", false, "Disable dry run mode")
	startCmd.Flags().Int64P("check-interval", "i", 120, "Check interval in seconds")
	startCmd.Flags().StringP("tag-name", "t", "ttl", "Set the tag name to check for deletion")

	// AWS
	startCmd.Flags().StringSliceP("aws-regions", "a", nil, "Set AWS regions")
	startCmd.Flags().BoolP("enable-eks", "e", false, "Enable EKS watch")
	startCmd.Flags().BoolP("enable-rds", "r", false, "Enable RDS watch")
	startCmd.Flags().BoolP("enable-documentdb", "m", false, "Enable DocumentDB watch")
	startCmd.Flags().BoolP("enable-elasticache", "c", false, "Enable Elasticache watch")
	startCmd.Flags().BoolP("enable-elb", "l", false, "Enable Elastic Load Balancers watch (true is eks is enabled)")
	startCmd.Flags().BoolP("enable-ebs", "b", false, "Enable Elastic Load Balancers watch (true is eks is enabled)")
	startCmd.Flags().BoolP("enable-vpc", "p", false, "Enable VPC watch")
	startCmd.Flags().BoolP("enable-s3", "s", false, "Enable S3 watch")
	startCmd.Flags().BoolP("enable-cloudwatch-logs", "w", false, "Enable Cloudwatch Logs watch")
	startCmd.Flags().BoolP("enable-kms", "n", false, "Enable KMS watch")
	startCmd.Flags().BoolP("enable-iam", "u", false, "Enable IAM watch")
	startCmd.Flags().BoolP("enable-ssh", "z", false, "Enable Key Pair watch")


	// K8s
	startCmd.Flags().StringP("kube-conn", "k", "off","Kubernetes connection method, choose between : off/in/out")
}