package aws

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type documentDBCluster struct {
	DBClusterIdentifier string
	DBClusterMembers    []string
	ClusterCreateTime   time.Time
	Status              string
	TTL                 int64
}

func listTaggedDocumentDBClusters(svc rds.RDS, tagName string) ([]documentDBCluster, error) {
	var taggedClusters []documentDBCluster
	var instances []string

	log.Debugf("Listing all DocumentDB clusters")
	// unfortunately AWS doesn't support tag filtering for RDS
	result, err := svc.DescribeDBClusters(nil)
	if err != nil {
		return nil, err
	}

	if len(result.DBClusters) == 0 {
		log.Debug("No DocumentDB clusters were found")
		return nil, nil
	}

	for _, cluster := range result.DBClusters {
		for _, tag := range cluster.TagList {
			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Key)
					continue
				}

				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on instance %s in %s",
						*tag.Value, *cluster.DBClusterIdentifier, *svc.Config.Region)
					continue
				}

				for _, instance := range cluster.DBClusterMembers {
					instances = append(instances, *instance.DBInstanceIdentifier)
				}

				taggedClusters = append(taggedClusters, documentDBCluster{
					DBClusterIdentifier: *cluster.DBClusterIdentifier,
					DBClusterMembers:    instances,
					ClusterCreateTime:   *cluster.ClusterCreateTime,
					Status:              *cluster.Status,
					TTL:                 int64(ttl),
				})
			}
		}
	}
	log.Debugf("Found %d DocumentDB cluster(s) in ready status with ttl tag", len(taggedClusters))

	return taggedClusters, nil
}

func deleteDocumentDBCluster(svc rds.RDS, cluster documentDBCluster, dryRun bool) error {
	deleteInstancesErrors := 0

	if cluster.Status == "deleting" {
		log.Infof("DocumentDB cluster %s is already in deletion process, skipping...", cluster.DBClusterIdentifier)
		return nil
	} else {
		log.Infof("Deleting DocumentDB cluster %s in %s, expired after %d seconds",
			cluster.DBClusterIdentifier, *svc.Config.Region, cluster.TTL)
	}

	// delete instance before deleting the cluster (otherwise it fails)
	for _, instance := range cluster.DBClusterMembers {
		rdsInstanceInfo, err := GetRDSInstanceInfos(svc, instance)
		if err != nil {
			log.Errorf("Can't access RDS instance %s information for DocumentDB cluster %s: %s",
				instance, cluster.DBClusterIdentifier, err)
			deleteInstancesErrors++
			continue
		}

		err = DeleteRDSDatabase(svc, rdsInstanceInfo, dryRun)
		if err != nil {
			log.Errorf("Deletion error on DocumentDB instance %s/%s/%s: %s",
				instance, cluster.DBClusterIdentifier, *svc.Config.Region, err)
			deleteInstancesErrors++
		}
	}

	if deleteInstancesErrors > 0 {
		message := fmt.Sprintf("Errors during deleting DocumentDB cluster %s instances, will try later when errors will be gone", cluster.DBClusterIdentifier)
		return errors.New(message)
	}

	if dryRun {
		return nil
	}

	// delete cluster
	_, err := svc.DeleteDBCluster(
		&rds.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(cluster.DBClusterIdentifier),
			SkipFinalSnapshot:   aws.Bool(true),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredDocumentDBClusters(svc rds.RDS, tagName string, dryRun bool) error {
	clusters, err := listTaggedDocumentDBClusters(svc, tagName)
	if err != nil {
		return fmt.Errorf("can't list DocumentDB databases: %s\n", err)
	}

	for _, cluster := range clusters {
		if CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL) {
			err := deleteDocumentDBCluster(svc, cluster, dryRun)
			if err != nil {
				log.Errorf("Deletion DocumentDB cluster error %s/%s: %s",
					cluster.DBClusterIdentifier, *svc.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("DocumentDB cluster %s in %s, has not yet expired",
				cluster.DBClusterIdentifier, *svc.Config.Region)
		}
	}

	return nil
}

// Todo: add subnet group delete support
