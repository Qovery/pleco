package aws

import (
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
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
	tagName, _ := cmd.Flags().GetString("tag-name")

	// AWS session
	currentSession, err := CreateSession(region)
	if err != nil {
		logrus.Errorf("AWS session error: %s", err)
	}

	// RDS + DocumentDB connection
	var currentRdsSession *rds.RDS
	rdsEnabled, _ := cmd.Flags().GetBool("enable-rds")
	documentdbEnabled, _ := cmd.Flags().GetBool("enable-documentdb")
	if rdsEnabled || documentdbEnabled {
		currentRdsSession = RdsSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))
	}

	// Elasticache connection
	var currentElasticacheSession *elasticache.ElastiCache
	elasticacheEnabled, _ := cmd.Flags().GetBool("enable-elasticache")
	if elasticacheEnabled {
		currentElasticacheSession = ElasticacheSession(*currentSession, os.Getenv("AWS_DEFAULT_REGION"))
	}

	// EKS connection
	var currentEKSSession *eks.EKS
	eksEnabled, _ := cmd.Flags().GetBool("enable-eks")
	if eksEnabled {
		currentEKSSession = eks.New(currentSession)
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
			err = DeleteExpiredEKSClusters(*currentEKSSession, tagName, dryRun)
			if err != nil {
				logrus.Error(err)
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

	wg.Wait()
}