package aws

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
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
}

func RdsSession(sess session.Session, region string) *rds.RDS {
	return rds.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedRDSDatabases(svc rds.RDS, tagName string) ([]rdsDatabase, error) {
	var taggedDatabases []rdsDatabase

	// unfortunately AWS doesn't support tag filtering for RDS
	result, err := svc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, err
	}

	if len(result.DBInstances) == 0 {
		return []rdsDatabase{}, nil
	}

	for _, instance := range result.DBInstances {
		if instance.TagList == nil {
			log.Errorf("No tags for instance %s in %s", *instance.DBInstanceIdentifier, *svc.Config.Region)
			continue
		}

		if instance.InstanceCreateTime == nil {
			log.Errorf("No creation date for instance %s in %s", *instance.DBInstanceIdentifier, *svc.Config.Region)
			continue
		}

		_, ttl, isProtected, _, _ := utils.GetEssentialTags(instance.TagList,tagName)
		time, _ := time.Parse(time.RFC3339, instance.InstanceCreateTime.Format(time.RFC3339))


		if instance.DBInstanceIdentifier != nil {
			taggedDatabases = append(taggedDatabases, rdsDatabase{
				DBInstanceIdentifier: *instance.DBInstanceIdentifier,
				InstanceCreateTime:   time,
				DBInstanceStatus:     *instance.DBInstanceStatus,
				TTL:                  int64(ttl),
				IsProtected: isProtected,
			})
		}
	}

	return taggedDatabases, nil
}

func DeleteRDSDatabase(svc rds.RDS, database rdsDatabase) error {
	if database.DBInstanceStatus == "deleting" {
		log.Infof("RDS instance %s is already in deletion process, skipping...", database.DBInstanceIdentifier)
		return nil
	} else {
		log.Infof("Deleting RDS database %s in %s, expired after %d seconds",
			database.DBInstanceIdentifier, *svc.Config.Region, database.TTL)
	}


	_, err := svc.DeleteDBInstance(
		&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier:      aws.String(database.DBInstanceIdentifier),
			DeleteAutomatedBackups:    aws.Bool(true),
			SkipFinalSnapshot:         aws.Bool(true),
		},
	)
	if err != nil {
		return err
	}

	return nil
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

func DeleteExpiredRDSDatabases(svc rds.RDS, tagName string, dryRun bool) {
	databases, err := listTaggedRDSDatabases(svc, tagName)
	region := svc.Config.Region
	if err != nil {
		log.Errorf("can't list RDS databases: %s\n", err)
		return
	}

	var expiredDatabases []rdsDatabase
	for _, database := range databases {
		if utils.CheckIfExpired(database.InstanceCreateTime, database.TTL, "rds Db: " + database.DBInstanceIdentifier) && !database.IsProtected {
			expiredDatabases = append(expiredDatabases, database)
		}
	}

	count, start:= utils.ElemToDeleteFormattedInfos("expired RDS database", len(expiredDatabases), *region)

	log.Debug(count)

	if dryRun || len(expiredDatabases) == 0 || expiredDatabases == nil{
		return
	}

	log.Debug(start)

	for _, database := range expiredDatabases {
		deletionErr := DeleteRDSDatabase(svc, database)
			if deletionErr != nil {
				log.Errorf("Deletion RDS database error %s/%s: %s",
					database.DBInstanceIdentifier, *svc.Config.Region, err)
			}
	}
}

func getRDSSubnetGroups(svc rds.RDS) []*rds.DBSubnetGroup {
	result, err := svc.DescribeDBSubnetGroups(
		&rds.DescribeDBSubnetGroupsInput{
			MaxRecords: aws.Int64(100),
		})

	if err != nil {
		log.Errorf("Can't get DocumentDB subnet groups in region %s: %s", *svc.Config.Region, err.Error())
	}

	return result.DBSubnetGroups
}

func getRDSSubnetGroupsTags(svc rds.RDS, dbSubnetGroupArn string) []*rds.Tag {
	result, err := svc.ListTagsForResource(
		&rds.ListTagsForResourceInput{
			ResourceName: aws.String(dbSubnetGroupArn),
		})

	if err != nil {
		log.Errorf("Can't get tags for %s in region %s: %s", dbSubnetGroupArn, *svc.Config.Region, err.Error())
		return []*rds.Tag{}
	}

	return result.TagList
}

func getExpiredRDSSubnetGroups(svc rds.RDS, tagName string) []*rds.DBSubnetGroup {
	RDSSubnetGroups := getRDSSubnetGroups(svc)
	var expiredRDSSubnetGroups []*rds.DBSubnetGroup

	for _, RDSSubnetGroup := range RDSSubnetGroups {
		tags := getRDSSubnetGroupsTags(svc, *RDSSubnetGroup.DBSubnetGroupArn)
		creationDate, ttl, isProtected, _, _ := utils.GetEssentialTags(tags, tagName)

		if utils.CheckIfExpired(creationDate, ttl, "RDS subnet group: " + *RDSSubnetGroup.DBSubnetGroupArn) && !isProtected {
			expiredRDSSubnetGroups = append(expiredRDSSubnetGroups, RDSSubnetGroup)
		}
	}

	return expiredRDSSubnetGroups
}

func deleteRDSSubnetGroup(svc rds.RDS, dbSubnetGroupName string) error {
	_, err := svc.DeleteDBSubnetGroup(
		&rds.DeleteDBSubnetGroupInput{
			DBSubnetGroupName: aws.String(dbSubnetGroupName),
		})

	if err != nil {
		return fmt.Errorf("Can't delete DocumentDB %s in region %s: %s", dbSubnetGroupName, *svc.Config.Region, err.Error())
	}

	return nil
}

func DeleteExpiredRDSSubnetGroups(svc rds.RDS, tagName string ,dryRun bool) {
	expiredRDSSubnetGroups :=  getExpiredRDSSubnetGroups(svc, tagName)

	count, start:= utils.ElemToDeleteFormattedInfos("expired RDS subnet group", len(expiredRDSSubnetGroups), *svc.Config.Region)

	log.Debug(count)

	if dryRun || len(expiredRDSSubnetGroups) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredRDSSubnetGroup := range expiredRDSSubnetGroups {
		err := deleteRDSSubnetGroup(svc, *expiredRDSSubnetGroup.DBSubnetGroupName)
		if err != nil {
			log.Errorf("Deletion RDS subnet group error %s/%s: %s", *expiredRDSSubnetGroup.DBSubnetGroupName, *svc.Config.Region, err)
		}
	}
}