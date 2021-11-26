package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"time"
)

type rdsDatabase struct {
	DBInstanceIdentifier string
	InstanceCreateTime   time.Time
	DBInstanceStatus     string
	TTL                  int64
	IsProtected          bool
	ParameterGroups      []*rds.DBParameterGroupStatus
	SubnetGroup          *rds.DBSubnetGroup
}

type RDSSubnetGroup struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
	Tag          string
}

func RdsSession(sess session.Session, region string) *rds.RDS {
	return rds.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listRDSDatabases(svc rds.RDS) []*rds.DBInstance {
	result, err := svc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})

	if err != nil {
		log.Errorf("Can't get RDS databases in %s: %s", *svc.Config.Region, err.Error())
		return []*rds.DBInstance{}
	}

	return result.DBInstances
}

func listExpiredRDSDatabases(svc rds.RDS, tagName string) []rdsDatabase {
	dbs := listRDSDatabases(svc)

	if len(dbs) == 0 {
		return nil
	}

	var expiredDatabases []rdsDatabase

	for _, instance := range dbs {
		if *instance.DBInstanceStatus == "deleting" {
			continue
		}

		if instance.TagList == nil {
			log.Errorf("No tags for instance %s in %s", *instance.DBInstanceIdentifier, *svc.Config.Region)
			continue
		}

		if instance.InstanceCreateTime == nil {
			log.Errorf("No creation date for instance %s in %s", *instance.DBInstanceIdentifier, *svc.Config.Region)
			continue
		}

		essentialTags := common.GetEssentialTags(instance.TagList, tagName)
		time, _ := time.Parse(time.RFC3339, instance.InstanceCreateTime.Format(time.RFC3339))

		if instance.DBInstanceIdentifier != nil {
			database := rdsDatabase{
				DBInstanceIdentifier: *instance.DBInstanceIdentifier,
				InstanceCreateTime:   time,
				DBInstanceStatus:     *instance.DBInstanceStatus,
				TTL:                  essentialTags.TTL,
				IsProtected:          essentialTags.IsProtected,
				SubnetGroup:          instance.DBSubnetGroup,
				ParameterGroups:      instance.DBParameterGroups,
			}

			if common.CheckIfExpired(database.InstanceCreateTime, database.TTL, "rds Db: "+database.DBInstanceIdentifier) && !database.IsProtected {
				expiredDatabases = append(expiredDatabases, database)
			}
		}
	}

	return expiredDatabases
}

func DeleteRDSDatabase(svc rds.RDS, database rdsDatabase) {
	if database.DBInstanceStatus == "deleting" {
		log.Infof("RDS instance %s is already in deletion process, skipping...", database.DBInstanceIdentifier)
		return
	} else {
		log.Infof("Deleting RDS database %s in %s, expired after %d seconds",
			database.DBInstanceIdentifier, *svc.Config.Region, database.TTL)
	}

	_, instanceErr := svc.DeleteDBInstance(
		&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier:   aws.String(database.DBInstanceIdentifier),
			DeleteAutomatedBackups: aws.Bool(true),
			SkipFinalSnapshot:      aws.Bool(true),
		},
	)
	if instanceErr != nil {
		log.Errorf("Can't delete RDS instance %s in %s: %s", database.DBInstanceIdentifier, *svc.Config.Region, instanceErr.Error())
	} else {
		DeleteRDSSubnetGroup(svc, *database.SubnetGroup.DBSubnetGroupName)

		for _, parameterGroup := range database.ParameterGroups {
			deleteRDSParameterGroups(svc, *parameterGroup.DBParameterGroupName)
		}
	}
}

func GetRDSInstanceInfos(svc rds.RDS, databaseIdentifier string) (rdsDatabase, error) {
	input := rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(databaseIdentifier),
	}

	result, err := svc.DescribeDBInstances(&input)
	// ignore if creation is in progress to avoid nil fields
	if err != nil || *result.DBInstances[0].DBInstanceStatus == "creating" {
		return rdsDatabase{
			DBInstanceIdentifier: *result.DBInstances[0].DBInstanceIdentifier,
			InstanceCreateTime:   time.Time{},
			DBInstanceStatus:     *result.DBInstances[0].DBInstanceStatus,
			TTL:                  0,
		}, err
	}

	time, _ := time.Parse(time.RFC3339, result.DBInstances[0].InstanceCreateTime.Format(time.RFC3339))

	return rdsDatabase{
		DBInstanceIdentifier: *result.DBInstances[0].DBInstanceIdentifier,
		InstanceCreateTime:   time,
		DBInstanceStatus:     *result.DBInstances[0].DBInstanceStatus,
		TTL:                  -1,
	}, nil
}

func DeleteExpiredRDSDatabases(sessions AWSSessions, options AwsOptions) {
	expiredDatabases := listExpiredRDSDatabases(*sessions.RDS, options.TagName)
	region := *sessions.RDS.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired RDS database", len(expiredDatabases), region)

	log.Debug(count)

	if options.DryRun || len(expiredDatabases) == 0 || expiredDatabases == nil {
		return
	}

	log.Debug(start)

	for _, database := range expiredDatabases {
		DeleteRDSDatabase(*sessions.RDS, database)
	}
}

func DeleteRDSSubnetGroup(svc rds.RDS, dbSubnetGroupName string) {
	_, err := svc.DeleteDBSubnetGroup(
		&rds.DeleteDBSubnetGroupInput{
			DBSubnetGroupName: aws.String(dbSubnetGroupName),
		})

	if err != nil {
		log.Errorf("Can't delete RDS Subnet Group %s in region %s: %s", dbSubnetGroupName, *svc.Config.Region, err.Error())
	}
}

func deleteRDSParameterGroups(svc rds.RDS, dbParameterGroupName string) {
	_, err := svc.DeleteDBParameterGroup(
		&rds.DeleteDBParameterGroupInput{
			DBParameterGroupName: aws.String(dbParameterGroupName),
		})

	if err != nil {
		log.Errorf("Can't delete RDS parameter group %s in region %s: %s", dbParameterGroupName, *svc.Config.Region, err.Error())
	}
}

func listRDSSubnetGroups(svc rds.RDS) []*rds.DBSubnetGroup {
	result, err := svc.DescribeDBSubnetGroups(
		&rds.DescribeDBSubnetGroupsInput{})

	if err != nil {
		log.Errorf("Can't list RDS subnet groups in region %s: %s", *svc.Config.Region, err.Error())
	}

	return result.DBSubnetGroups
}

func getRDSSubnetGroupTags(svc rds.RDS, subnetGroupName string) []*rds.Tag {
	result, err := svc.ListTagsForResource(
		&rds.ListTagsForResourceInput{ResourceName: aws.String(subnetGroupName)})

	if err != nil {
		log.Errorf("Can't get RDS subnet groups tags for %s: %s", subnetGroupName, err.Error())
	}

	return result.TagList
}

func getExpiredRDSSubnetGroups(svc rds.RDS, tagName string) []RDSSubnetGroup {
	SGs := listRDSSubnetGroups(svc)

	expiredRDSSubnetGroups := []RDSSubnetGroup{}
	for _, SG := range SGs {
		tags := getRDSSubnetGroupTags(svc, *SG.DBSubnetGroupArn)
		essentialTags := common.GetEssentialTags(tags, tagName)
		if common.CheckIfExpired(essentialTags.CreationDate, essentialTags.TTL, "DB subnet group"+*SG.DBSubnetGroupName) && !essentialTags.IsProtected {
			expiredRDSSubnetGroups = append(expiredRDSSubnetGroups, RDSSubnetGroup{
				ID:           *SG.DBSubnetGroupArn,
				Name:         *SG.DBSubnetGroupName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				IsProtected:  essentialTags.IsProtected,
				Tag:          essentialTags.Tag,
			})
		}
	}

	return expiredRDSSubnetGroups
}

func DeleteExpiredRDSSubnetGroups(sessions AWSSessions, options AwsOptions) {
	region := *sessions.RDS.Config.Region
	expiredRDSSubnetGroups := getExpiredRDSSubnetGroups(*sessions.RDS, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired RDS subnet group", len(expiredRDSSubnetGroups), region)

	log.Debug(count)

	if options.DryRun || len(expiredRDSSubnetGroups) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredRDSSubnetGroup := range expiredRDSSubnetGroups {
		DeleteRDSSubnetGroup(*sessions.RDS, expiredRDSSubnetGroup.Name)
	}
}

type RDSParameterGroups struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
	Tag          string
}

func listParametersGroups(svc rds.RDS) []*rds.DBParameterGroup {
	results, err := svc.DescribeDBParameterGroups(&rds.DescribeDBParameterGroupsInput{})

	if err != nil {
		log.Errorf("Can't get RDS Parameter Groups in %s: %s", *svc.Config.Region, err.Error())
		return nil
	}

	return results.DBParameterGroups
}

func getCompleteRDSParameterGroups(svc rds.RDS, tagName string) []RDSParameterGroups {
	results := listParametersGroups(svc)

	completeRDSParameterGroups := []RDSParameterGroups{}

	for _, result := range results {
		tags, tagsErr := svc.ListTagsForResource(&rds.ListTagsForResourceInput{ResourceName: aws.String(*result.DBParameterGroupArn)})

		if tagsErr != nil {
			log.Errorf("Can't get RDS Parameter Groups Tags in %s: %s", *svc.Config.Region, tagsErr.Error())
			return completeRDSParameterGroups
		}

		essentialTags := common.GetEssentialTags(tags.TagList, tagName)

		completeRDSParameterGroups = append(completeRDSParameterGroups, RDSParameterGroups{
			ID:           *result.DBParameterGroupArn,
			Name:         *result.DBParameterGroupName,
			CreationDate: essentialTags.CreationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
			Tag:          essentialTags.Tag,
		})
	}

	return completeRDSParameterGroups
}

func listExpiredCompleteRDSParameterGroups(svc rds.RDS, tagName string) []RDSParameterGroups {
	completeRDSParameterGroups := getCompleteRDSParameterGroups(svc, tagName)
	expiredCompleteRDSParameterGroups := []RDSParameterGroups{}

	for _, item := range completeRDSParameterGroups {
		if common.CheckIfExpired(item.CreationDate, item.TTL, "DB Parameter Group"+item.Name) && !item.IsProtected {
			expiredCompleteRDSParameterGroups = append(expiredCompleteRDSParameterGroups, item)
		}
	}

	return expiredCompleteRDSParameterGroups
}

func DeleteExpiredCompleteRDSParameterGroups(sessions AWSSessions, options AwsOptions) {
	expiredRDSParameterGroups := listExpiredCompleteRDSParameterGroups(*sessions.RDS, options.TagName)
	region := *sessions.RDS.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired RDS Parameter Group", len(expiredRDSParameterGroups), region)

	log.Debug(count)

	if options.DryRun || len(expiredRDSParameterGroups) == 0 || expiredRDSParameterGroups == nil {
		return
	}

	log.Debug(start)

	for _, dbParameterGroup := range expiredRDSParameterGroups {
		deleteRDSParameterGroups(*sessions.RDS, dbParameterGroup.Name)
	}
}

func listSnapshots(svc rds.RDS) []*rds.DBClusterSnapshot {
	result, err := svc.DescribeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{})

	if err != nil {
		log.Errorf("Can't list RDS snapshots in region %s: %s", *svc.Config.Region, err.Error())
	}

	return result.DBClusterSnapshots
}

func getExpiredSnapshots(svc rds.RDS) []*rds.DBClusterSnapshot {
	dbs := listRDSDatabases(svc)
	snaps := listSnapshots(svc)

	expiredSnaps := []*rds.DBClusterSnapshot{}

	if len(dbs) == 0 {
		for _, snap := range snaps {
			if snap.SnapshotCreateTime.Before(time.Now().UTC().Add(3 * time.Hour)) {
				expiredSnaps = append(expiredSnaps, snap)
			}
		}

		return expiredSnaps
	}

	snapsChecking := make(map[string]*rds.DBClusterSnapshot)
	for _, snap := range snaps {
		snapsChecking[*snap.DBClusterIdentifier] = snap
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

func deleteSnapshot(svc rds.RDS, snapName string) {
	_, err := svc.DeleteDBSnapshot(&rds.DeleteDBSnapshotInput{DBSnapshotIdentifier: aws.String(snapName)})

	if err != nil {
		log.Errorf("Can't delete RDS snapshot %s in region %s: %s", snapName, *svc.Config.Region, err.Error())
	}
}

func DeleteExpiredSnapshots(sessions AWSSessions, options AwsOptions) {
	expiredSnapshots := getExpiredSnapshots(*sessions.RDS)
	region := *sessions.RDS.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired RDS snapshot", len(expiredSnapshots), region)

	log.Debug(count)

	if options.DryRun || len(expiredSnapshots) == 0 || expiredSnapshots == nil {
		return
	}

	log.Debug(start)

	for _, snapshot := range expiredSnapshots {
		deleteSnapshot(*sessions.RDS, *snapshot.DBClusterSnapshotIdentifier)
	}
}
