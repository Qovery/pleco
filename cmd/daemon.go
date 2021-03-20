package cmd

import (
	"github.com/Qovery/pleco/providers/aws"
	"github.com/Qovery/pleco/providers/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sync"
)

func StartDaemon(disableDryRun bool, interval int64, cmd *cobra.Command) {
	var wg sync.WaitGroup
	dryRun := !disableDryRun
	if dryRun {
		log.Info("Dry run mode enabled")
	}

	checkEnvVars(cmd)

	// run Kubernetes check
	k8s.RunPlecoKubernetes(cmd, interval, dryRun, &wg)

	// run AWS checks
	go startAWS(cmd, interval, dryRun, &wg)
	wg.Wait()
}

func startAWS(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	regions, _ := cmd.Flags().GetStringSlice("aws-regions")
	awsOptions := &aws.AwsOption{
		DryRun:               dryRun,
		TagName:              getCmdString(cmd, "tag-name"),
		EnableRDS:            getCmdBool(cmd, "enable-rds"),
		EnableDocumentDB:     getCmdBool(cmd, "enable-documentdb"),
		EnableElastiCache:    getCmdBool(cmd, "enable-elasticache"),
		EnableEKS:            getCmdBool(cmd, "enable-eks"),
		EnableELB:            getCmdBool(cmd, "enable-elb"),
		EnableEBS:            getCmdBool(cmd, "enable-ebs"),
		EnableVPC:            getCmdBool(cmd, "enable-vpc"),
		EnableS3:             getCmdBool(cmd, "enable-s3"),
		EnableCloudWatchLogs: getCmdBool(cmd, "enable-cloudwatch-logs"),
		EnableKMS:            getCmdBool(cmd, "enable-kms"),
		EnableIAM:            getCmdBool(cmd, "enable-iam"),
		EnableSSH:            getCmdBool(cmd, "enable-ssh"),
	}
	for err := range aws.RunPlecoAWS(regions, interval, awsOptions) {
		if err != nil {
			log.Errorf("error starting aws: %s", err.Error())
		}
	}
	wg.Done()
}

func getCmdString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func getCmdBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}
