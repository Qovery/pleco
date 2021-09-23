package pkg

import (
	"github.com/Qovery/pleco/third_party/aws"
	"github.com/Qovery/pleco/third_party/k8s"
	"github.com/Qovery/pleco/third_party/scaleway"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"
	"sync"
)

func StartDaemon(cloudProvider string, disableDryRun bool, interval int64, cmd *cobra.Command) {
	var wg sync.WaitGroup
	dryRun := true
	if disableDryRun {
		dryRun = false
	} else {
		log.Info("Dry run mode enabled")
	}

	log.Infof("Cloud provider: %s", strings.ToUpper(cloudProvider))


	checkEnvVars(cloudProvider, cmd)

	k8s.RunPlecoKubernetes(cmd, interval, dryRun, &wg)

	run(cloudProvider, dryRun, interval, cmd, &wg)

	wg.Wait()
}

func run(cloudProvider string, dryRun bool, interval int64, cmd *cobra.Command, wg *sync.WaitGroup) {
	switch cloudProvider {
	case "aws":
		startAWS(cmd, interval, dryRun, wg)
	case "scaleway":
		startScaleway(cmd, interval, dryRun, wg)
	default:
		log.Fatalf("Unknown cloud provider: %s", cloudProvider)
	}
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
	aws.RunPlecoAWS(cmd, regions, interval, wg, awsOptions)
	wg.Done()
}

func startScaleway(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	regions, _ := cmd.Flags().GetStringSlice("scaleway-regions")
	scalewayOptions := &scaleway.ScalewayOption{
		DryRun:        dryRun,
		TagName:       getCmdString(cmd, "tag-name"),
		EnableCluster: getCmdBool(cmd, "enable-cluster"),
		EnableDB:      getCmdBool(cmd, "enable-db"),
		EnableCR:      getCmdBool(cmd, "enable-cr"),
	}
	scaleway.RunPlecoScaleway(regions, interval, wg, scalewayOptions)
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
