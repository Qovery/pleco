package aws

import (
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type AwsOptions struct {
	TagName                string
	TagValue               string
	DisableTTLCheck        bool
	IsDestroyingCommand    bool
	DryRun                 bool
	EnableRDS              bool
	EnableElastiCache      bool
	EnableEKS              bool
	EnableELB              bool
	EnableEBS              bool
	EnableVPC              bool
	EnableS3               bool
	EnableCloudWatchLogs   bool
	EnableKMS              bool
	EnableIAM              bool
	EnableSSH              bool
	EnableDocumentDB       bool
	EnableECR              bool
	EnableSQS              bool
	EnableLambda           bool
	EnableSFN              bool
	EnableCloudFormation   bool
	EnableEC2Instance      bool
	EnableCloudWatchEvents bool
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
	SQS            *sqs.SQS
	LambdaFunction *lambda.Lambda
	SFN            *sfn.SFN
	CloudFormation *cloudformation.CloudFormation
	EventBridge    *eventbridge.EventBridge
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
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredElasticacheDatabases, DeleteUnlinkedECSubnetGroups, DeleteExpiredElasticacheSnapshots)
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
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredVPC, DeleteExpiredElasticIps, DeleteExpiredNatGateways)
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
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredRepositories)
	}

	// SQS
	if options.EnableSQS {
		sessions.SQS = sqs.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredSQSQueues)
	}

	// Cloudwatch events
	if options.EnableCloudWatchEvents {
		sessions.EventBridge = eventbridge.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredCloudWatchEvents)
	}

	// Lambda Function
	if options.EnableLambda {
		sessions.LambdaFunction = lambda.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredLambdaFunctions)
	}

	// Step Function State Machines
	if options.EnableSFN {
		sessions.SFN = sfn.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredStateMachines)
	}

	// Cloudformation Stacks
	if options.EnableCloudFormation {
		sessions.CloudFormation = cloudformation.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredStacks)
	}

	if options.EnableEC2Instance {
		sessions.EC2 = ec2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteExpiredEC2Instances)
	}

	if options.DisableTTLCheck {
		sessions.EC2 = ec2.New(currentSession)
		sessions.ELB = elbv2.New(currentSession)
		listServiceToCheckStatus = append(listServiceToCheckStatus, DeleteVPCLinkedResourcesWithQuota)
	}

	if options.IsDestroyingCommand {
		for _, check := range listServiceToCheckStatus {
			check(sessions, options)
		}
	} else {
		for {
			for _, check := range listServiceToCheckStatus {
				check(sessions, options)
			}

			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

func runPlecoInGlobal(cmd *cobra.Command, interval int64, wg *sync.WaitGroup, currentSession *session.Session, options AwsOptions) {
	defer wg.Done()

	logrus.Info("Starting to check global expired resources.")

	sessions := AWSSessions{}

	// IAM
	iamEnabled, _ := cmd.Flags().GetBool("enable-iam")
	if iamEnabled {
		sessions.IAM = iam.New(currentSession)
	}

	if options.IsDestroyingCommand {
		deleteExpiredIAM(iamEnabled, &sessions, &options)
	} else {
		for {
			deleteExpiredIAM(iamEnabled, &sessions, &options)
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}
}

func deleteExpiredIAM(iamEnabled bool, sessions *AWSSessions, options *AwsOptions) {
	// check IAM
	if iamEnabled {
		logrus.Debug("Listing all IAM access.")
		DeleteExpiredIAM(sessions, options)
	}
}
