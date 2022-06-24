package pkg

import (
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Qovery/pleco/pkg/aws"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/Qovery/pleco/pkg/do"
	"github.com/Qovery/pleco/pkg/k8s"
	"github.com/Qovery/pleco/pkg/scaleway"
)

func StartDaemon(cloudProvider string, disableDryRun bool, interval int64, cmd *cobra.Command) {
	var wg sync.WaitGroup
	dryRun := true
	if disableDryRun {
		dryRun = false
		log.Warn("Dry run mode disabled")
	} else {
		log.Info("Dry run mode enabled")
	}

	log.Infof("Cloud provider: %s", strings.ToUpper(cloudProvider))

	common.CheckEnvVars(cloudProvider, cmd)

	k8s.RunPlecoKubernetes(cmd, interval, dryRun, &wg)

	run(cloudProvider, dryRun, interval, cmd, &wg)

	wg.Wait()
}

func StartDestroy(cloudProvider string, disableDryRun bool, cmd *cobra.Command) {
	var wg sync.WaitGroup
	dryRun := true
	if disableDryRun {
		dryRun = false
		log.Warn("Dry run mode disabled")
	} else {
		log.Info("Dry run mode enabled")
	}
	log.Infof("Cloud provider: %s", strings.ToUpper(cloudProvider))

	common.CheckEnvVars(cloudProvider, cmd)

	run(cloudProvider, dryRun, 0, cmd, &wg)

	wg.Wait()
}

func run(cloudProvider string, dryRun bool, interval int64, cmd *cobra.Command, wg *sync.WaitGroup) {
	switch cloudProvider {
	case "aws":
		startAWS(cmd, interval, dryRun, wg)
	case "scaleway":
		startScaleway(cmd, interval, dryRun, wg)
	case "do":
		startDO(cmd, interval, dryRun, wg)
	default:
		log.Fatalf("Unknown cloud provider: %s", cloudProvider)
	}
}

func startAWS(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	regions, _ := cmd.Flags().GetStringSlice("aws-regions")
	tagValue := getCmdString(cmd, "tag-value")

	awsOptions := aws.AwsOptions{
		DryRun:               dryRun,
		TagName:              getCmdString(cmd, "tag-name"),
		TagValue:             tagValue,
		IsDestroyingCommand:  strings.TrimSpace(tagValue) != "",
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
		EnableECR:            getCmdBool(cmd, "enable-ecr"),
		EnableSQS:            getCmdBool(cmd, "enable-sqs"),
		EnableLambda:         getCmdBool(cmd, "enable-lambda"),
		EnableSFN:            getCmdBool(cmd, "enable-sfn"),
		EnableCloudFormation: getCmdBool(cmd, "enable-cloudformation"),
		EnableEC2Instance:    getCmdBool(cmd, "enable-ec2-instance"),
	}
	aws.RunPlecoAWS(cmd, regions, interval, wg, awsOptions)
	wg.Done()
}

func startScaleway(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	zones, _ := cmd.Flags().GetStringSlice("scw-zones")
	tagValue := getCmdString(cmd, "tag-value")

	scalewayOptions := scaleway.ScalewayOptions{
		TagName:             getCmdString(cmd, "tag-name"),
		TagValue:            tagValue,
		IsDestroyingCommand: strings.TrimSpace(tagValue) != "",
		DryRun:              dryRun,
		EnableCluster:       getCmdBool(cmd, "enable-cluster"),
		EnableDB:            getCmdBool(cmd, "enable-db"),
		EnableCR:            getCmdBool(cmd, "enable-cr"),
		EnableBucket:        getCmdBool(cmd, "enable-s3"),
		EnableLB:            getCmdBool(cmd, "enable-lb"),
		EnableVolume:        getCmdBool(cmd, "enable-volume"),
		EnableSG:            getCmdBool(cmd, "enable-sg"),
	}
	scaleway.RunPlecoScaleway(zones, interval, wg, scalewayOptions)
	wg.Done()
}

func startDO(cmd *cobra.Command, interval int64, dryRun bool, wg *sync.WaitGroup) {
	regions, _ := cmd.Flags().GetStringSlice("do-regions")
	tagValue := getCmdString(cmd, "tag-value")

	DOOptions := do.DOOptions{
		TagName:             getCmdString(cmd, "tag-name"),
		TagValue:            tagValue,
		IsDestroyingCommand: strings.TrimSpace(tagValue) != "",
		DryRun:              dryRun,
		EnableCluster:       getCmdBool(cmd, "enable-cluster"),
		EnableDB:            getCmdBool(cmd, "enable-db"),
		EnableBucket:        getCmdBool(cmd, "enable-s3"),
		EnableLB:            getCmdBool(cmd, "enable-lb"),
		EnableVolume:        getCmdBool(cmd, "enable-volume"),
		EnableFirewall:      getCmdBool(cmd, "enable-firewall"),
		EnableVPC:           getCmdBool(cmd, "enable-vpc"),
	}
	do.RunPlecoDO(regions, interval, wg, DOOptions)
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
