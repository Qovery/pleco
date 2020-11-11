package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type rdsDatabase struct {
	DBInstanceArn string
	DBInstanceIdentifier string
	InstanceCreateTime time.Time
	TTL int
}

func ListTaggedDatabases(sess session.Session, region string, tag string) ([]rdsDatabase, error) {
	var taggedDatabases []rdsDatabase
	svc := rds.New(&sess, &aws.Config{Region: aws.String(region)})

	log.Debugf("listing RDS databases")
	// unfortunately AWS doesn't support tag filtering for RDS
	result, err := svc.DescribeDBInstances(nil)
	if err != nil {
		return nil, err
	}

	if len(result.DBInstances) == 0 {
		log.Debug("no RDS instances were found")
		return nil, nil
	}

	log.Debugf("found %d instance(s), filtering on tag \"%s\"\n", len(result.DBInstances), tag)
	for _, instance := range result.DBInstances {
		for _, tagName := range instance.TagList {
			if *tagName.Key == tag {
				log.Debugf("checking tag %s", *tagName.Key)
				if *tagName.Key == "" {
					log.Warn("tag %s was empty, skipping", tag)
					continue
				}
				ttl, err := strconv.Atoi(*tagName.Value)
				if err == nil {
					log.Errorf("error while trying to convert tag value to integer: %s", err)
				}
				taggedDatabases = append(taggedDatabases, rdsDatabase{
					DBInstanceArn:        *instance.DBInstanceArn,
					DBInstanceIdentifier: *instance.DBClusterIdentifier,
					InstanceCreateTime:   *instance.InstanceCreateTime,
					TTL:                  ttl,
				})
			}
		}
	}

	log.Infof("%s", result)
	return taggedDatabases, nil
}