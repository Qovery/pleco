package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type documentDBCluster struct {
	common.CloudProviderResource
	DBClusterMembers []string
	SubnetGroupName  string
	Status           string
}

func getDBClusters(svc rds.RDS, tagName string) []documentDBCluster {
	result, err := svc.DescribeDBClusters(&rds.DescribeDBClustersInput{})
	if err != nil {
		log.Errorf("Can't get DB clusters in %s: %s", *svc.Config.Region, err.Error())
		return nil
	}

	var dbClusters []documentDBCluster
	for _, cluster := range result.DBClusters {
		var dbClusterMembers []string
		for _, instance := range cluster.DBClusterMembers {
			dbClusterMembers = append(dbClusterMembers, *instance.DBInstanceIdentifier)
		}

		essentialTags := common.GetEssentialTags(cluster.TagList, tagName)
		time, _ := time.Parse(time.RFC3339, cluster.ClusterCreateTime.Format(time.RFC3339))

		dbClusters = append(dbClusters, documentDBCluster{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *cluster.DBClusterIdentifier,
				Description:  "Document DB: " + *cluster.DBClusterIdentifier,
				CreationDate: time,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			DBClusterMembers: dbClusterMembers,
			SubnetGroupName:  *cluster.DBSubnetGroup,
			Status:           *cluster.Status,
		})
	}

	return dbClusters
}

func listExpiredDocumentDBClusters(svc rds.RDS, options *AwsOptions) []documentDBCluster {
	dbClusters := getDBClusters(svc, options.TagName)

	var expiredClusters []documentDBCluster

	for _, cluster := range dbClusters {
		if cluster.IsResourceExpired(options.TagValue) {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	return expiredClusters
}

func deleteClusterInstances(svc rds.RDS, cluster documentDBCluster) {
	for _, instance := range cluster.DBClusterMembers {
		rdsInstanceInfo, err := GetRDSInstanceInfos(svc, instance)
		if err != nil {
			log.Errorf("Can't access RDS instance %s information for DocumentDB cluster %s: %s",
				instance, cluster.Identifier, err)
			continue
		}

		DeleteRDSDatabase(svc, rdsInstanceInfo)
	}
}

func deleteDocumentDBCluster(svc rds.RDS, cluster documentDBCluster, dryRun bool) error {
	if cluster.Status == "deleting" {
		log.Infof("DocumentDB cluster %s is already in deletion process, skipping...", cluster.Identifier)
		return nil
	} else {
		log.Infof("Deleting DocumentDB cluster %s in %s, expired after %d seconds",
			cluster.Identifier, *svc.Config.Region, cluster.TTL)
	}

	if dryRun {
		return nil
	}
	// delete instance before deleting the cluster (otherwise it fails)
	deleteClusterInstances(svc, cluster)

	// delete cluster
	_, err := svc.DeleteDBCluster(
		&rds.DeleteDBClusterInput{
			DBClusterIdentifier: aws.String(cluster.Identifier),
			SkipFinalSnapshot:   aws.Bool(true),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredDocumentDBClusters(sessions AWSSessions, options AwsOptions) {
	region := *sessions.RDS.Config.Region
	expiredClusters := listExpiredDocumentDBClusters(*sessions.RDS, &options)

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
				cluster.Identifier, region, deletionErr.Error())
		}
	}
}

func listClusterSnapshots(svc rds.RDS) []*rds.DBClusterSnapshot {
	result, err := svc.DescribeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{SnapshotType: aws.String("manual")})

	if err != nil {
		log.Errorf("Can't list RDS snapshots in region %s: %s", *svc.Config.Region, err.Error())
	}

	return result.DBClusterSnapshots
}

func getExpiredClusterSnapshots(svc rds.RDS, options *AwsOptions) []*rds.DBClusterSnapshot {
	dbs := listRDSDatabases(svc)
	snaps := listClusterSnapshots(svc)

	expiredSnaps := []*rds.DBClusterSnapshot{}

	if len(dbs) == 0 {
		for _, snap := range snaps {
			if common.CheckClusterSnapshot(snap) &&
				(options.IsDestroyingCommand || snap.SnapshotCreateTime.Before(time.Now().UTC().Add(3*time.Hour)) && common.CheckClusterSnapshot(snap)) {
				expiredSnaps = append(expiredSnaps, snap)
			}
		}

		return expiredSnaps
	}

	snapsChecking := make(map[string]*rds.DBClusterSnapshot)
	for _, snap := range snaps {
		if common.CheckClusterSnapshot(snap) {
			snapsChecking[*snap.DBClusterSnapshotIdentifier] = snap
		}
	}

	for _, db := range dbs {
		if db.DBClusterIdentifier != nil {
			snapsChecking[*db.DBClusterIdentifier] = nil
		}
	}

	for _, snap := range snapsChecking {
		if snap != nil {
			expiredSnaps = append(expiredSnaps, snap)
		}
	}

	return expiredSnaps
}

func deleteClusterSnapshot(svc rds.RDS, snapName string) {
	_, err := svc.DeleteDBClusterSnapshot(&rds.DeleteDBClusterSnapshotInput{DBClusterSnapshotIdentifier: aws.String(snapName)})

	if err != nil {
		log.Errorf("Can't delete RDS snapshot %s in region %s: %s", snapName, *svc.Config.Region, err.Error())
	}
}

func DeleteExpiredClusterSnapshots(sessions AWSSessions, options AwsOptions) {
	expiredSnapshots := getExpiredClusterSnapshots(*sessions.RDS, &options)
	region := *sessions.RDS.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired RDS cluster snapshot", len(expiredSnapshots), region)

	log.Debug(count)

	if options.DryRun || len(expiredSnapshots) == 0 || expiredSnapshots == nil {
		return
	}

	log.Debug(start)

	for _, snapshot := range expiredSnapshots {
		deleteClusterSnapshot(*sessions.RDS, *snapshot.DBClusterSnapshotIdentifier)
	}
}
