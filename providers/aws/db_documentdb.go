package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"time"
)

type documentDBCluster struct {
	DBClusterIdentifier string
	DBClusterMembers    []string
	ClusterCreateTime   time.Time
	Status              string
	TTL                 int64
	IsProtected         bool
}

func getDBClusters(svc rds.RDS) []*rds.DBCluster {
	result, err := svc.DescribeDBClusters(&rds.DescribeDBClustersInput{})
	if err != nil {
		log.Errorf("Can't get DB clusters in %s: %s", *svc.Config.Region, err.Error())
		return nil
	}

	return result.DBClusters
}

func listExpiredDocumentDBClusters(svc rds.RDS, tagName string) []documentDBCluster {
	dbClusters := getDBClusters(svc)

	var expiredClusters []documentDBCluster

	for _, cluster := range dbClusters {
		var instances []string
		for _, instance := range cluster.DBClusterMembers {
			instances = append(instances, *instance.DBInstanceIdentifier)
		}

		_, ttl, isProtected, _, _ := utils.GetEssentialTags(cluster.TagList, tagName)
		time, _ := time.Parse(time.RFC3339, cluster.ClusterCreateTime.Format(time.RFC3339))

		if utils.CheckIfExpired(time, ttl, "document Db: "+*cluster.DBClusterIdentifier) && !isProtected {
			expiredClusters = append(expiredClusters, documentDBCluster{
				DBClusterIdentifier: *cluster.DBClusterIdentifier,
				DBClusterMembers:    instances,
				ClusterCreateTime:   time,
				Status:              *cluster.Status,
				TTL:                 ttl,
				IsProtected:         isProtected,
			})
		}
	}

	return expiredClusters
}

func deleteClusterInstances(svc rds.RDS, cluster documentDBCluster) {
	for _, instance := range cluster.DBClusterMembers {
		rdsInstanceInfo, err := GetRDSInstanceInfos(svc, instance)
		if err != nil {
			log.Errorf("Can't access RDS instance %s information for DocumentDB cluster %s: %s",
				instance, cluster.DBClusterIdentifier, err)
			continue
		}

		DeleteRDSDatabase(svc, rdsInstanceInfo)
	}
}

func deleteDocumentDBCluster(svc rds.RDS, cluster documentDBCluster, dryRun bool) error {
	if cluster.Status == "deleting" {
		log.Infof("DocumentDB cluster %s is already in deletion process, skipping...", cluster.DBClusterIdentifier)
		return nil
	} else {
		log.Infof("Deleting DocumentDB cluster %s in %s, expired after %d seconds",
			cluster.DBClusterIdentifier, *svc.Config.Region, cluster.TTL)
	}

	if dryRun {
		return nil
	}
	// delete instance before deleting the cluster (otherwise it fails)
	deleteClusterInstances(svc, cluster)

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

func DeleteExpiredDocumentDBClusters(svc rds.RDS, tagName string, dryRun bool) {
	expiredClusters := listExpiredDocumentDBClusters(svc, tagName)

	count, start := utils.ElemToDeleteFormattedInfos("expired DocumentDB database", len(expiredClusters), *svc.Config.Region)

	log.Debug(count)

	if dryRun || len(expiredClusters) == 0 {
		return
	}

	log.Debug(start)

	for _, cluster := range expiredClusters {
		deletionErr := deleteDocumentDBCluster(svc, cluster, dryRun)
		if deletionErr != nil {
			log.Errorf("Deletion DocumentDB cluster error %s in %s: %s",
				cluster.DBClusterIdentifier, *svc.Config.Region, deletionErr.Error())
		}
	}

}
