package aws

import (
	"errors"
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"time"
)

type documentDBCluster struct {
	DBClusterIdentifier string
	DBClusterMembers []string
	ClusterCreateTime time.Time
	Status string
	TTL int64
	IsProtected bool
}

func listTaggedDocumentDBClusters(svc rds.RDS, tagName string) ([]documentDBCluster, error) {
	var taggedClusters []documentDBCluster
	var instances []string

	// unfortunately AWS doesn't support tag filtering for RDS
	result, err := svc.DescribeDBClusters(nil)
	if err != nil {
		return nil, err
	}

	if len(result.DBClusters) == 0 {
		return nil, nil
	}

	for _, cluster := range result.DBClusters {
		for _, instance := range cluster.DBClusterMembers {
			instances = append(instances, *instance.DBInstanceIdentifier)
		}

		_, ttl, isProtected, _, _ := utils.GetEssentialTags(cluster.TagList,tagName)

		taggedClusters = append(taggedClusters, documentDBCluster{
			DBClusterIdentifier:  *cluster.DBClusterIdentifier,
			DBClusterMembers: 	  instances,
			ClusterCreateTime:    *cluster.ClusterCreateTime,
			Status:               *cluster.Status,
			TTL:                  ttl,
			IsProtected: 		  isProtected,
		})
	}

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

		if dryRun {
			continue
		}

		err = DeleteRDSDatabase(svc, rdsInstanceInfo)
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
			DBClusterIdentifier:       aws.String(cluster.DBClusterIdentifier),
			SkipFinalSnapshot:         aws.Bool(true),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredDocumentDBClusters(svc rds.RDS, tagName string, dryRun bool) {
	clusters, err := listTaggedDocumentDBClusters(svc, tagName)
	region := svc.Config.Region
	if err != nil {
		log.Errorf("can't list DocumentDB databases: %s\n", err)
		return
	}

	var expiredClusters []documentDBCluster
	for _, cluster := range clusters {
		if utils.CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL, "document Db: " + cluster.DBClusterIdentifier)  && !cluster.IsProtected {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	count, start:= utils.ElemToDeleteFormattedInfos("expired DocumentDB database", len(expiredClusters), *region)

	log.Debug(count)

	if dryRun || len(expiredClusters) == 0 {
		return
	}

	log.Debug(start)


	for _, cluster := range expiredClusters {
		deletionErr := deleteDocumentDBCluster(svc, cluster, dryRun)
		if deletionErr != nil {
			log.Errorf("Deletion DocumentDB cluster error %s/%s: %s",
				cluster.DBClusterIdentifier, *svc.Config.Region, err)
		}
	}

}
