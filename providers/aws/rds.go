package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type rdsDatabase struct {
	DBInstanceIdentifier string
	InstanceCreateTime time.Time
	DBInstanceStatus string
	TTL int64
}

func RdsSession(sess session.Session, region string) *rds.RDS {
	return rds.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedDatabases(svc rds.RDS, tagName string) ([]rdsDatabase, error) {
	var taggedDatabases []rdsDatabase

	log.Debugf("Listing all RDS databases")
	// unfortunately AWS doesn't support tag filtering for RDS
	result, err := svc.DescribeDBInstances(nil)
	if err != nil {
		return nil, err
	}

	if len(result.DBInstances) == 0 {
		log.Debug("No RDS instances were found")
		return nil, nil
	}

	log.Debugf("Found %d RDS instance(s), filtering on tag \"%s\"\n", len(result.DBInstances), tagName)
	for _, instance := range result.DBInstances {
		for _, tag := range instance.TagList {
			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warn("Tag %s was empty and it wasn't expected, skipping", tag)
					continue
				}

				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on instance %s in %s",
						*tag.Value, *instance.DBInstanceIdentifier, svc.Config.Region)
					continue
				}

				taggedDatabases = append(taggedDatabases, rdsDatabase{
					DBInstanceIdentifier: *instance.DBInstanceIdentifier,
					InstanceCreateTime:   *instance.InstanceCreateTime,
					DBInstanceStatus:     *instance.DBInstanceStatus,
					TTL:                  int64(ttl),
				})
			}
		}
	}
	log.Debugf("Found %d RDS instance(s) with ttl tag", len(taggedDatabases))

	return taggedDatabases, nil
}

func deleteDatabase(svc rds.RDS, database rdsDatabase, dryRun bool) error {
	if database.DBInstanceStatus == "deleting" {
		log.Infof("RDS instance %s is already in deletion process, skipping...", database.DBInstanceIdentifier)
		return nil
	} else {
		log.Infof("Deleting RDS database %s in %s, expired after %d seconds",
			database.DBInstanceIdentifier, *svc.Config.Region, database.TTL)
	}

	if dryRun {
		return nil
	}

	_, err := svc.DeleteDBInstance(
		&rds.DeleteDBInstanceInput {
			DBInstanceIdentifier: aws.String(database.DBInstanceIdentifier),
			SkipFinalSnapshot: aws.Bool(true),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getRDSInstanceInfos(svc rds.RDS, databaseIdentifier string) (rdsDatabase, error) {
	input := rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(databaseIdentifier),
	}

	result, err := svc.DescribeDBInstances(&input)
	if err != nil {
		return rdsDatabase{}, err
	}

	return rdsDatabase{
		DBInstanceIdentifier: *result.DBInstances[0].DBInstanceIdentifier,
		InstanceCreateTime:   *result.DBInstances[0].InstanceCreateTime,
		DBInstanceStatus:     *result.DBInstances[0].DBInstanceStatus,
		TTL:                  -1,
	}, nil
}

func DeleteExpiredDatabases(svc rds.RDS, tagName string, dryRun bool) {
	databases, _ := listTaggedDatabases(svc, tagName)
	for _, database := range databases {
		if utils.CheckIfExpired(database.InstanceCreateTime, database.TTL) {
			err := deleteDatabase(svc, database, dryRun)
			if err != nil {
				log.Errorf("Deletion RDS database error %s/%s: %s",
					database.DBInstanceIdentifier, *svc.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("RDS database %s in %s, has not yet expired",
				database.DBInstanceIdentifier, *svc.Config.Region)
		}
	}
}