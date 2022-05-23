package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type rdsDatabase struct {
	common.CloudProviderResource
	DBInstanceStatus string
	ParameterGroups  []*rds.DBParameterGroupStatus
	SubnetGroup      *rds.DBSubnetGroup
}

type RDSSubnetGroup struct {
	common.CloudProviderResource
	ID string
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

func listExpiredRDSDatabases(svc rds.RDS, options *AwsOptions) []rdsDatabase {
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

		essentialTags := common.GetEssentialTags(instance.TagList, options.TagName)
		time, _ := time.Parse(time.RFC3339, instance.InstanceCreateTime.Format(time.RFC3339))

		if instance.DBInstanceIdentifier != nil {
			database := rdsDatabase{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   *instance.DBInstanceIdentifier,
					Description:  "RDS Database: " + *instance.DBInstanceIdentifier,
					CreationDate: time,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				DBInstanceStatus: *instance.DBInstanceStatus,
				SubnetGroup:      instance.DBSubnetGroup,
				ParameterGroups:  instance.DBParameterGroups,
			}
			if database.CloudProviderResource.IsResourceExpired(options.TagValue) {
				expiredDatabases = append(expiredDatabases, database)
			}
		}
	}

	return expiredDatabases
}

func DeleteRDSDatabase(svc rds.RDS, database rdsDatabase) {
	if database.DBInstanceStatus == "deleting" {
		log.Infof("RDS instance %s is already in deletion process, skipping...", database.Identifier)
		return
	} else {
		log.Infof("Deleting RDS database %s in %s, expired after %d seconds",
			database.Identifier, *svc.Config.Region, database.TTL)
	}

	_, instanceErr := svc.DeleteDBInstance(
		&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier:   aws.String(database.Identifier),
			DeleteAutomatedBackups: aws.Bool(true),
			SkipFinalSnapshot:      aws.Bool(true),
		},
	)
	if instanceErr != nil {
		log.Errorf("Can't delete RDS instance %s in %s: %s", database.Identifier, *svc.Config.Region, instanceErr.Error())
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
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *result.DBInstances[0].DBInstanceIdentifier,
				Description:  "RDS Database: " + *result.DBInstances[0].DBInstanceIdentifier,
				CreationDate: time.Time{},
				TTL:          0,
				Tag:          "",
				IsProtected:  false,
			},
			DBInstanceStatus: *result.DBInstances[0].DBInstanceStatus,
		}, err
	}

	time, _ := time.Parse(time.RFC3339, result.DBInstances[0].InstanceCreateTime.Format(time.RFC3339))

	return rdsDatabase{
		CloudProviderResource: common.CloudProviderResource{
			Identifier:   *result.DBInstances[0].DBInstanceIdentifier,
			Description:  "RDS Database: " + *result.DBInstances[0].DBInstanceIdentifier,
			CreationDate: time,
			TTL:          -1,
			Tag:          "",
			IsProtected:  false,
		},
		DBInstanceStatus: *result.DBInstances[0].DBInstanceStatus,
	}, nil
}

func DeleteExpiredRDSDatabases(sessions AWSSessions, options AwsOptions) {
	expiredDatabases := listExpiredRDSDatabases(*sessions.RDS, &options)
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

func getExpiredRDSSubnetGroups(svc rds.RDS, options *AwsOptions) []RDSSubnetGroup {
	SGs := listRDSSubnetGroups(svc)

	expiredRDSSubnetGroups := []RDSSubnetGroup{}
	for _, SG := range SGs {
		tags := getRDSSubnetGroupTags(svc, *SG.DBSubnetGroupArn)
		essentialTags := common.GetEssentialTags(tags, options.TagName)
		rDSSubnetGroup := RDSSubnetGroup{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *SG.DBSubnetGroupName,
				Description:  "DB Subnet Group: " + *SG.DBSubnetGroupName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			ID: *SG.DBSubnetGroupArn,
		}
		if rDSSubnetGroup.IsResourceExpired(options.TagValue) {
			expiredRDSSubnetGroups = append(expiredRDSSubnetGroups)
		}
	}

	return expiredRDSSubnetGroups
}

func DeleteExpiredRDSSubnetGroups(sessions AWSSessions, options AwsOptions) {
	region := *sessions.RDS.Config.Region
	expiredRDSSubnetGroups := getExpiredRDSSubnetGroups(*sessions.RDS, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired RDS subnet group", len(expiredRDSSubnetGroups), region)

	log.Debug(count)

	if options.DryRun || len(expiredRDSSubnetGroups) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredRDSSubnetGroup := range expiredRDSSubnetGroups {
		DeleteRDSSubnetGroup(*sessions.RDS, expiredRDSSubnetGroup.Identifier)
	}
}

type RDSParameterGroups struct {
	common.CloudProviderResource
	ID string
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
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *result.DBParameterGroupName,
				Description:  "RDS Parameter Group: " + *result.DBParameterGroupName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			ID: *result.DBParameterGroupArn,
		})
	}

	return completeRDSParameterGroups
}

func listExpiredCompleteRDSParameterGroups(svc rds.RDS, options *AwsOptions) []RDSParameterGroups {
	completeRDSParameterGroups := getCompleteRDSParameterGroups(svc, options.TagName)
	expiredCompleteRDSParameterGroups := []RDSParameterGroups{}

	for _, item := range completeRDSParameterGroups {
		if item.IsResourceExpired(options.TagValue) {
			expiredCompleteRDSParameterGroups = append(expiredCompleteRDSParameterGroups, item)
		}
	}

	return expiredCompleteRDSParameterGroups
}

func DeleteExpiredCompleteRDSParameterGroups(sessions AWSSessions, options AwsOptions) {
	expiredRDSParameterGroups := listExpiredCompleteRDSParameterGroups(*sessions.RDS, &options)
	region := *sessions.RDS.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired RDS Parameter Group", len(expiredRDSParameterGroups), region)

	log.Debug(count)

	if options.DryRun || len(expiredRDSParameterGroups) == 0 || expiredRDSParameterGroups == nil {
		return
	}

	log.Debug(start)

	for _, dbParameterGroup := range expiredRDSParameterGroups {
		deleteRDSParameterGroups(*sessions.RDS, dbParameterGroup.Identifier)
	}
}

func listSnapshots(svc rds.RDS) []*rds.DBSnapshot {
	result, err := svc.DescribeDBSnapshots(&rds.DescribeDBSnapshotsInput{SnapshotType: aws.String("manual")})

	if err != nil {
		log.Errorf("Can't list RDS snapshots in region %s: %s", *svc.Config.Region, err.Error())
	}

	return result.DBSnapshots
}

func getExpiredSnapshots(svc rds.RDS) []*rds.DBSnapshot {
	dbs := listRDSDatabases(svc)
	snaps := listSnapshots(svc)

	expiredSnaps := []*rds.DBSnapshot{}

	if len(dbs) == 0 {
		for _, snap := range snaps {
			// do we need to force delete every snapshot on detroy command ?
			if snap.SnapshotCreateTime.Before(time.Now().UTC().Add(3*time.Hour)) && common.CheckSnapshot(snap) {
				expiredSnaps = append(expiredSnaps, snap)
			}
		}

		return expiredSnaps
	}

	snapsChecking := make(map[string]*rds.DBSnapshot)
	for _, snap := range snaps {
		if common.CheckSnapshot(snap) {
			snapsChecking[*snap.DBSnapshotIdentifier] = snap
		}
		snapsChecking[*snap.DBSnapshotIdentifier] = snap
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
		deleteSnapshot(*sessions.RDS, *snapshot.DBSnapshotIdentifier)
	}
}
