package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"time"
)

type documentDBCluster struct {
	DBClusterIdentifier string
	DBClusterMembers    []string
	ClusterCreateTime   time.Time
	SubnetGroupName     string
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

		essentialTags := common.GetEssentialTags(cluster.TagList, tagName)
		time, _ := time.Parse(time.RFC3339, cluster.ClusterCreateTime.Format(time.RFC3339))

		if common.CheckIfExpired(time, essentialTags.TTL, "document Db: "+*cluster.DBClusterIdentifier) && !essentialTags.IsProtected {
			expiredClusters = append(expiredClusters, documentDBCluster{
				DBClusterIdentifier: *cluster.DBClusterIdentifier,
				DBClusterMembers:    instances,
				ClusterCreateTime:   time,
				SubnetGroupName:     *cluster.DBSubnetGroup,
				Status:              *cluster.Status,
				TTL:                 essentialTags.TTL,
				IsProtected:         essentialTags.IsProtected,
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

func DeleteExpiredDocumentDBClusters(sessions *AWSSessions, options *AwsOptions) {
	region := *sessions.RDS.Config.Region
	expiredClusters := listExpiredDocumentDBClusters(*sessions.RDS, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired DocumentDB database", len(expiredClusters), region)

	log.Debug(count)

	if options.DryRun || len(expiredClusters) == 0 {
		return
	}

	log.Debug(start)

	for _, cluster := range expiredClusters {
		DeleteRDSSubnetGroup(*sessions.RDS, cluster.SubnetGroupName)
		deletionErr := deleteDocumentDBCluster(*sessions.RDS, cluster, options.DryRun)
		if deletionErr != nil {
			log.Errorf("Deletion DocumentDB cluster error %s in %s: %s",
				cluster.DBClusterIdentifier, region, deletionErr.Error())
		}
	}
}
