package aws

import (
	// "time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/sirupsen/logrus"
	// log "github.com/sirupsen/logrus"
)

type sqsQueue struct {
	QueueName  string
	// ReplicationGroupId string
	// ClusterCreateTime  time.Time
	// ClusterStatus      string
	// SubnetGroup        string
	// TTL                int64
	// IsProtected        bool
}

func SqsSession(sess session.Session, region string) *sqs.SQS {
	return sqs.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedSqsQueues(svc sqs.SQS, tagName string) ([]sqsQueue, error) {
	var taggedQueues []sqsQueue

	result, err := svc.ListQueues(nil)
	if err != nil {
		return nil, err
	}

	if len(result.QueueUrls) == 0 {
		return nil, nil
	}

	for _, queue := range result.QueueUrls {
		tags, err := svc.ListQueueTags(
			&sqs.ListQueueTagsInput{
				QueueUrl: aws.String(*queue),
			},
		)
		if err != nil {
			continue
		}


		essentialTags := common.GetEssentialTags(tags.Tags, tagName)
		logrus.Info("These are the queue tag: %s", essentialTags)
	// 	time, _ := time.Parse(time.RFC3339, cluster.CacheClusterCreateTime.Format(time.RFC3339))
	// 	taggedQueues = append(taggedQueues, elasticacheCluster{
	// 		ClusterIdentifier:  *cluster.CacheClusterId,
	// 		ReplicationGroupId: replicationGroupId,
	// 		ClusterCreateTime:  time,
	// 		ClusterStatus:      *cluster.CacheClusterStatus,
	// 		SubnetGroup:        *cluster.CacheSubnetGroupName,
	// 		TTL:                essentialTags.TTL,
	// 		IsProtected:        essentialTags.IsProtected,
	// 	})

	}

	return taggedQueues, nil
}

// func deleteElasticacheCluster(svc elasticache.ElastiCache, cluster elasticacheCluster) error {
// 	if cluster.ClusterStatus == "deleting" {
// 		log.Infof("Elasticache cluster %s is already in deletion process, skipping...", cluster.ClusterIdentifier)
// 		return nil
// 	} else {
// 		log.Infof("Deleting Elasticache cluster %s in %s, expired after %d seconds",
// 			cluster.ClusterIdentifier, *svc.Config.Region, cluster.TTL)
// 	}

// 	// with replicas
// 	if cluster.ReplicationGroupId != "" {
// 		_, err := svc.DeleteReplicationGroup(
// 			&elasticache.DeleteReplicationGroupInput{
// 				ReplicationGroupId:   aws.String(cluster.ReplicationGroupId),
// 				RetainPrimaryCluster: aws.Bool(false),
// 			},
// 		)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	_, err := svc.DeleteCacheCluster(
// 		&elasticache.DeleteCacheClusterInput{
// 			CacheClusterId: aws.String(cluster.ClusterIdentifier),
// 		},
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func getExpiredClusters(ECsession *elasticache.ElastiCache, tagName string) ([]elasticacheCluster, string) {
// 	clusters, err := listTaggedElasticacheDatabases(*ECsession, tagName)
// 	region := *ECsession.Config.Region
// 	if err != nil {
// 		log.Errorf("can't list Elasticache databases in region %s: %s", region, err.Error())
// 	}

// 	var expiredClusters []elasticacheCluster
// 	for _, cluster := range clusters {
// 		if common.CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL, "elasticache: "+cluster.ClusterIdentifier) && !cluster.IsProtected {
// 			expiredClusters = append(expiredClusters, cluster)
// 		}
// 	}

// 	return expiredClusters, region
// }

// func DeleteExpiredElasticacheDatabases(sessions AWSSessions, options AwsOptions) {
// 	expiredClusters, region := getExpiredClusters(sessions.ElastiCache, options.TagName)

// 	count, start := common.ElemToDeleteFormattedInfos("expired Elasticache database", len(expiredClusters), region)

// 	log.Debug(count)

// 	if options.DryRun || len(expiredClusters) == 0 {
// 		return
// 	}

// 	log.Debug(start)

// 	for _, cluster := range expiredClusters {
// 		deleteECSubnetGroups(sessions.ElastiCache, cluster.SubnetGroup)
// 		deletionErr := deleteElasticacheCluster(*sessions.ElastiCache, cluster)
// 		if deletionErr != nil {
// 			log.Errorf("Deletion Elasticache cluster error %s/%s: %s", cluster.ClusterIdentifier, region, deletionErr.Error())
// 		}
// 	}
// }

// func deleteECSubnetGroups(ECsession *elasticache.ElastiCache, subnetGroupName string) {
// 	_, err := ECsession.DeleteCacheSubnetGroup(
// 		&elasticache.DeleteCacheSubnetGroupInput{
// 			CacheSubnetGroupName: aws.String(subnetGroupName),
// 		},
// 	)

// 	if err != nil {
// 		log.Errorf("Can't delete elasticache subnet group %s: %s", subnetGroupName, err.Error())
// 	}
// }

// func getECSubnetGroups(ECsession *elasticache.ElastiCache) []*elasticache.CacheSubnetGroup {
// 	result, err := ECsession.DescribeCacheSubnetGroups(
// 		&elasticache.DescribeCacheSubnetGroupsInput{})

// 	if err != nil {
// 		log.Errorf("Can't list elasticache subnet groups: %s", err.Error())
// 	}

// 	return result.CacheSubnetGroups
// }

// func getUnlinkedSubnetGroupNames(ECsession *elasticache.ElastiCache, ec2Session *ec2.EC2) []string {
// 	subnetGroups := getECSubnetGroups(ECsession)
// 	VPCs := GetAllVPCs(ec2Session)

// 	comp := make(map[string]*string)

// 	for _, subnetGroup := range subnetGroups {
// 		comp[*subnetGroup.VpcId] = subnetGroup.CacheSubnetGroupName
// 	}

// 	for _, VPC := range VPCs {
// 		comp[*VPC.VpcId] = nil
// 	}

// 	var unlinkedSubenetGroupNames []string
// 	for _, subnetGroupName := range comp {
// 		if subnetGroupName != nil {
// 			unlinkedSubenetGroupNames = append(unlinkedSubenetGroupNames, *subnetGroupName)
// 		}
// 	}

// 	return unlinkedSubenetGroupNames
// }

// func DeleteUnlinkedECSubnetGroups(sessions AWSSessions, options AwsOptions) {
// 	unlinkedSubnetGroupNames := getUnlinkedSubnetGroupNames(sessions.ElastiCache, sessions.EC2)

// 	count, start := common.ElemToDeleteFormattedInfos("unliked Elasticache subnet group", len(unlinkedSubnetGroupNames), *sessions.ElastiCache.Config.Region)

// 	log.Debug(count)

// 	if options.DryRun || len(unlinkedSubnetGroupNames) == 0 {
// 		return
// 	}

// 	log.Debug(start)

// 	for _, unlinkedSubnetGroupName := range unlinkedSubnetGroupNames {
// 		deleteECSubnetGroups(sessions.ElastiCache, unlinkedSubnetGroupName)
// 	}
// }
