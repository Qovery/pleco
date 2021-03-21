package aws

import (
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

type AwsOption struct {
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
}

func RunPlecoAWS(regions []string, interval int64, options *AwsOption) chan error {
	c := make(chan error)
	var wg sync.WaitGroup
	for _, region := range regions {
		wg.Add(1)
		go func() {
			if err := runPlecoInRegion(region, interval, &wg, options); err != nil {
				c <- err
			}
		}()
	}
	go func() {
		wg.Wait()
		close(c)
	}()
	return c
}

func runPlecoInRegion(region string, interval int64, wg *sync.WaitGroup, options *AwsOption) error {
	defer wg.Done()

	sessions := &AWSSessions{}

	// AWS session
	currentSession, err := CreateSession(region)
	if err != nil {
		logrus.Errorf("AWS session error: %s", err)
	}

	if options.EnableRDS || options.EnableDocumentDB {
		options.EnableRDS = true
		sessions.RDS = RdsSession(*currentSession, region)
	}

	if options.EnableElastiCache {
		sessions.ElastiCache = ElasticacheSession(*currentSession, region)
	}

	// EKS connection
	if options.EnableEKS {
		sessions.EKS = eks.New(currentSession)
		sessions.ELB = elbv2.New(currentSession)
		options.EnableELB = true
		sessions.EC2 = ec2.New(currentSession)
		options.EnableEBS = true
		sessions.CloudWatchLogs = cloudwatchlogs.New(currentSession)
	}

	// ELB connection
	if options.EnableELB {
		sessions.ELB = elbv2.New(currentSession)
	}

	// EBS connection (VPC + SSH)
	if options.EnableEBS || options.EnableVPC || options.EnableSSH {
		options.EnableEBS = true
		sessions.EC2 = ec2.New(currentSession)
	}

	// S3
	if options.EnableS3 {
		sessions.S3 = s3.New(currentSession)
	}

	// Cloudwatch
	if options.EnableCloudWatchLogs {
		sessions.CloudWatchLogs = cloudwatchlogs.New(currentSession)
	}

	// KMS
	if options.EnableKMS {
		sessions.KMS = kms.New(currentSession)
	}

	// IAM
	if options.EnableIAM {
		sessions.IAM = iam.New(currentSession)
	}
	listServiceToCheckStatus := []struct {
		Active bool
		Func   funcDeleteExpired
	}{
		{Active: options.EnableRDS, Func: DeleteExpiredRDSDatabases},
		{Active: options.EnableDocumentDB, Func: DeleteExpiredDocumentDBClusters},
		{Active: options.EnableElastiCache, Func: DeleteExpiredElasticacheDatabases},
		{Active: options.EnableEKS, Func: DeleteExpiredEKSClusters},
		{Active: options.EnableELB, Func: DeleteExpiredLoadBalancers},
		{Active: options.EnableELB, Func: DeleteExpiredVolumes},
		{Active: options.EnableVPC, Func: DeleteExpiredVPC},
		{Active: options.EnableS3, Func: DeleteExpiredBuckets},
		{Active: options.EnableCloudWatchLogs, Func: DeleteExpiredLogs},
		{Active: options.EnableKMS, Func: DeleteKMSExpiredKeys},
		{Active: options.EnableIAM, Func: DeleteExpiredIAM},
		{Active: options.EnableSSH, Func: DeleteSSHExpiredKeys},
	}
	for {
		for _, l := range listServiceToCheckStatus {
			if l.Active {
				if err := l.Func(sessions, options); err != nil {
					// TODO : return error ?? will be added to chan error
					logrus.Errorf("error: %s", err.Error())
				}
			}
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return nil
}

type funcDeleteExpired func(sessions *AWSSessions, options *AwsOption) error
