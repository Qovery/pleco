package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type elasticacheCluster struct {
	ClusterIdentifier  string
	ReplicationGroupId string
	ClusterCreateTime  time.Time
	ClusterStatus      string
	TTL                int64
}

func ElasticacheSession(sess session.Session, region string) *elasticache.ElastiCache {
	return elasticache.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedElasticacheDatabases(svc *elasticache.ElastiCache, tagName string) ([]elasticacheCluster, error) {
	var taggedClusters []elasticacheCluster

	log.Debugf("Listing all Elasticache clusters")
	result, err := svc.DescribeCacheClusters(nil)
	if err != nil {
		return nil, err
	}

	if len(result.CacheClusters) == 0 {
		log.Debug("No Elasticache clusters were found")
		return nil, nil
	}

	for _, cluster := range result.CacheClusters {
		tags, err := svc.ListTagsForResource(
			&elasticache.ListTagsForResourceInput{
				ResourceName: aws.String(*cluster.ARN),
			},
		)
		if err != nil {
			if *cluster.CacheClusterStatus == "available" {
				log.Errorf("Can't get tags for Elasticache cluster: %s", *cluster.CacheClusterId)
			}
			continue
		}

		for _, tag := range tags.TagList {
			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Key)
					continue
				}

				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on instance %s in %s",
						*tag.Value, *cluster.CacheClusterId, *svc.Config.Region)
					continue
				}

				// required for replicas deletion
				replicationGroupId := ""
				if cluster.ReplicationGroupId != nil {
					replicationGroupId = *cluster.ReplicationGroupId
				}

				taggedClusters = append(taggedClusters, elasticacheCluster{
					ClusterIdentifier:  *cluster.CacheClusterId,
					ReplicationGroupId: replicationGroupId,
					ClusterCreateTime:  *cluster.CacheClusterCreateTime,
					ClusterStatus:      *cluster.CacheClusterStatus,
					TTL:                int64(ttl),
				})
			}
		}
	}
	log.Debugf("Found %d Elasticache cluster(s) in ready status with ttl tag", len(taggedClusters))

	return taggedClusters, nil
}

func deleteElasticacheCluster(svc *elasticache.ElastiCache, cluster elasticacheCluster, dryRun bool) error {
	if cluster.ClusterStatus == "deleting" {
		log.Infof("Elasticache cluster %s is already in deletion process, skipping...", cluster.ClusterIdentifier)
		return nil
	} else {
		log.Infof("Deleting Elasticache cluster %s in %s, expired after %d seconds",
			cluster.ClusterIdentifier, *svc.Config.Region, cluster.TTL)
	}

	if dryRun {
		return nil
	}

	// with replicas
	if cluster.ReplicationGroupId != "" {
		_, err := svc.DeleteReplicationGroup(
			&elasticache.DeleteReplicationGroupInput{
				ReplicationGroupId:   aws.String(cluster.ReplicationGroupId),
				RetainPrimaryCluster: aws.Bool(false),
			},
		)
		if err != nil {
			return err
		}
	}

	_, err := svc.DeleteCacheCluster(
		&elasticache.DeleteCacheClusterInput{
			CacheClusterId: aws.String(cluster.ClusterIdentifier),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredElasticacheDatabases(sessions *AWSSessions, options *AwsOption) error {
	clusters, err := listTaggedElasticacheDatabases(sessions.ElastiCache, options.TagName)
	if err != nil {
		return fmt.Errorf("can't list Elasticache databases: %s\n", err)
	}

	for _, cluster := range clusters {
		if CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL) {
			err := deleteElasticacheCluster(sessions.ElastiCache, cluster, options.DryRun)
			if err != nil {
				log.Errorf("Deletion Elasticache cluster error %s/%s: %s",
					cluster.ClusterIdentifier, *sessions.ElastiCache.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("Elasticache cluster %s in %s, has not yet expired",
				cluster.ClusterIdentifier, *sessions.ElastiCache.Config.Region)
		}
	}

	return nil
}
