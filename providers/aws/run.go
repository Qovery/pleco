package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sync"
	"time"
)

var wg sync.WaitGroup

func RunPlecoAWS(cmd *cobra.Command, regions []string, interval int64, dryRun bool) {
	for _, region := range regions {
		wg.Add(1)
		go runPlecoInRegion(cmd, region, interval, dryRun)
	}
}

func runPlecoInRegion(cmd *cobra.Command, region string, interval int64, dryRun bool) {
	defer wg.Done()

	var currentRdsSession *rds.RDS
	var currentElasticacheSession *elasticache.ElastiCache
	var currentEKSSession *eks.EKS
	var currentElbSession *elbv2.ELBV2
	var currentEC2Session *ec2.EC2
	elbEnabled := false
	ebsEnabled := false
	vpcEnabled := false

	tagName, _ := cmd.Flags().GetString("tag-name")

	// AWS session
	currentSession, err := CreateSession(region)
	if err != nil {
		logrus.Errorf("AWS session error: %s", err)
	}

	// RDS + DocumentDB connection
	rdsEnabled, _ := cmd.Flags().GetBool("enable-rds")
	documentdbEnabled, _ := cmd.Flags().GetBool("enable-documentdb")
	if rdsEnabled || documentdbEnabled {
		currentRdsSession = RdsSession(*currentSession, region)
	}

	// Elasticache connection
	elasticacheEnabled, _ := cmd.Flags().GetBool("enable-elasticache")
	if elasticacheEnabled {
		currentElasticacheSession = ElasticacheSession(*currentSession, region)
	}

	// EKS connection
	eksEnabled, _ := cmd.Flags().GetBool("enable-eks")
	if eksEnabled {
		currentEKSSession = eks.New(currentSession)
		currentElbSession = elbv2.New(currentSession)
		elbEnabled = true
		currentEC2Session = ec2.New(currentSession)
		ebsEnabled = true
	}

	// ELB connection
	elbEnabledByUser, _ := cmd.Flags().GetBool("enable-elb")
	if elbEnabled || elbEnabledByUser {
		currentElbSession = elbv2.New(currentSession)
		elbEnabled = true
	}

	// ELB connection
	ebsEnabledByUser, _ := cmd.Flags().GetBool("enable-ebs")
	if ebsEnabled || ebsEnabledByUser {
		currentEC2Session = ec2.New(currentSession)
		ebsEnabled = true
	}

	// VPC
	vpcEnabled, _ = cmd.Flags().GetBool("enable-vpc")
	if vpcEnabled {
		currentEC2Session = ec2.New(currentSession)
	}

	for {
		// check RDS
		if rdsEnabled {
			err = DeleteExpiredRDSDatabases(*currentRdsSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check DocumentDB
		if documentdbEnabled {
			err = DeleteExpiredDocumentDBClusters(*currentRdsSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check Elasticache
		if elasticacheEnabled {
			err = DeleteExpiredElasticacheDatabases(*currentElasticacheSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check EKS
		if eksEnabled {
			err = DeleteExpiredEKSClusters(*currentEKSSession, *currentEC2Session, *currentElbSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check load balancers
		if elbEnabled {
			err = DeleteExpiredLoadBalancers(*currentElbSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check EBS volumes
		if ebsEnabled {
			err = DeleteExpiredVolumes(*currentEC2Session, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		// check VPC
		if vpcEnabled {
			err = DeleteExpiredVPC(*currentEC2Session, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

	wg.Wait()
}