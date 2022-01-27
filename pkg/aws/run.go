package aws

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type AwsOptions struct {
	TagName              string
	DryRun               bool
	EnableRDS            bool
	EnableElastiCache    bool
	EnableEKS            bool
	EnableELB            bool
	EnableEBS            bool
	EnableVPC            bool
	EnableS3             bool
	EnableCloudWatchLogs bool
	EnableKMS            bool
	EnableIAM            bool
	EnableSSH            bool
	EnableDocumentDB     bool
	EnableECR            bool
	EnableSQS			 bool
}

type AWSSessions struct {
	RDS            *rds.RDS
	ElastiCache    *elasticache.ElastiCache
	EKS            *eks.EKS
	ELB            *elbv2.ELBV2
	EC2            *ec2.EC2
	S3             *s3.S3
	CloudWatchLogs *cloudwatchlogs.CloudWatchLogs
	KMS            *kms.KMS
	IAM            *iam.IAM
	ECR            *ecr.ECR
	SQS 		   *sqs.SQS
}

type funcDeleteExpired func(sessions AWSSessions, options AwsOptions)

func RunPlecoAWS(cmd *cobra.Command, regions []string, interval int64, wg *sync.WaitGroup, options AwsOptions) {
	for _, region := range regions {
		wg.Add(1)
		go runPlecoInRegion(region, interval, wg, options)
	}

	currentSession := CreateSession(regions[0])

	wg.Add(1)
	go runPlecoInGlobal(cmd, interval, wg, currentSession, options)
}

func runPlecoInRegion(region string, interval int64, wg *sync.WaitGroup, options AwsOptions) {
	defer wg.Done()

	sessions := AWSSessions{}
	currentSession := CreateSession(region)

	logrus.Infof("Starting to check expired resources in region %s.", *currentSession.Config.Region)

	var listServiceToCheckStatus []funcDeleteExpired

	// S3
	if options.EnableS3 {
		sessions.S3 = s3.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredBuckets)
	}

	// RDS
	if options.EnableRDS {
		sessions.RDS = RdsSession(*currentSession, region)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredRDSDatabases, DeleteExpiredRDSSubnetGroups, DeleteExpiredCompleteRDSParameterGroups, DeleteExpiredSnapshots)
	}

	// DocumentDB connection
	if options.EnableDocumentDB {
		sessions.RDS = RdsSession(*currentSession, region)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredDocumentDBClusters, DeleteExpiredClusterSnapshots)
	}

	// Elasticache connection
	if options.EnableElastiCache {
		sessions.ElastiCache = ElasticacheSession(*currentSession, region)
		sessions.EC2 = ec2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredElasticacheDatabases, DeleteUnlinkedECSubnetGroups)
	}

	// EKS connection
	if options.EnableEKS {
		sessions.EKS = eks.New(currentSession)
		sessions.ELB = elbv2.New(currentSession)
		options.EnableELB = true
		sessions.EC2 = ec2.New(currentSession)
		options.EnableEBS = true
		sessions.CloudWatchLogs = cloudwatchlogs.New(currentSession)
		sessions.RDS = RdsSession(*currentSession, region)

		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredEKSClusters)
	}

	// ELB connection
	if options.EnableELB {
		sessions.EKS = eks.New(currentSession)
		sessions.ELB = elbv2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLoadBalancers)
	}

	// EBS connection
	if options.EnableEBS {
		sessions.EKS = eks.New(currentSession)
		sessions.EC2 = ec2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVolumes)
	}

	// VPC
	if options.EnableVPC {
		sessions.EC2 = ec2.New(currentSession)
		sessions.ELB = elbv2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPC, DeleteExpiredElasticIps)
	}

	// Cloudwatch
	if options.EnableCloudWatchLogs {
		sessions.CloudWatchLogs = cloudwatchlogs.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLogs, DeleteUnlinkedLogs)
	}

	// KMS
	if options.EnableKMS {
		sessions.KMS = kms.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredKeys)
	}

	// SSH
	if options.EnableSSH {
		sessions.EC2 = ec2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredKeyPairs)
	}

	// ECR
	if options.EnableECR {
		sessions.ECR = ecr.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteEmptyRepositories)
	}

	// SQS
	if options.EnableSQS {
		sessions.SQS = sqs.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredSQSQueues)
	}

	for {
		for _, check := range listServiceToCheckStatus {
			check(sessions, options)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

}

func runPlecoInGlobal(cmd *cobra.Command, interval int64, wg *sync.WaitGroup, currentSession *session.Session, options AwsOptions) {
	defer wg.Done()

	logrus.Info("Starting to check global expired resources.")

	var currentIAMSession *iam.IAM

	// IAM
	iamEnabled, _ := cmd.Flags().GetBool("enable-iam")
	if iamEnabled {
		currentIAMSession = iam.New(currentSession)
	}

	for {
		// check IAM
		if iamEnabled {
			logrus.Debug("Listing all IAM access.")
			DeleteExpiredIAM(currentIAMSession, options.TagName, options.DryRun)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
