package aws

import (
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

// Cluster without ttl tag leaves resources TTL check never deletes once EKS cluster gone.
// Grace period: Terraform creates these before EKS cluster exists.
const orphanGracePeriod = 24 * time.Hour

func listLiveEKSClusterNames(svc *eks.EKS) ([]string, error) {
	clusters, err := ListClusters(*svc)

	return aws.StringValueSlice(clusters), err
}

func isOrphanOfDeletedCluster(resource common.CloudProviderResource, clusterId string, liveClusterNames []string, now time.Time) bool {
	if clusterId == "" || resource.IsProtected || resource.CreationDate.Year() < 1972 {
		return false
	}

	if now.Sub(resource.CreationDate) < orphanGracePeriod {
		return false
	}

	for _, name := range liveClusterNames {
		if strings.Contains(name, clusterId) {
			return false
		}
	}

	return true
}

func isQoveryClusterEventRule(name string) bool {
	return strings.HasPrefix(name, "qovery-cw-event-")
}

func isQoveryClusterQueue(queueUrl string, clusterId string) bool {
	return path.Base(queueUrl) == "qovery-"+clusterId
}

func isQoveryClusterSubnetGroup(name string) bool {
	return strings.HasPrefix(name, "vpc-") || strings.HasPrefix(name, "documentdb-vpc-")
}

// False in destroy mode or on EKS listing error: partial list would mark live clusters' resources orphaned.
func liveClusterNamesForSweep(sessions AWSSessions, options AwsOptions, region string) ([]string, bool) {
	if options.IsDestroyingCommand {
		return nil, false
	}

	names, err := listLiveEKSClusterNames(sessions.EKS)
	if err != nil {
		log.Errorf("Can't list EKS clusters in %s, skipping orphaned cluster resources: %s", region, err)
		return nil, false
	}

	return names, true
}

func DeleteOrphanedCloudWatchEvents(sessions AWSSessions, options AwsOptions) {
	region := *sessions.EventBridge.Config.Region
	liveClusterNames, ok := liveClusterNamesForSweep(sessions, options, region)
	if !ok {
		return
	}

	rules, err := listTaggedCloudWatchEvents(*sessions.EventBridge, options.TagName)
	if err != nil {
		log.Errorf("Can't list CloudWatch events in %s: %s", region, err)
		return
	}

	now := time.Now().UTC()
	var orphaned []cloudWatchEvent
	for _, rule := range rules {
		if isQoveryClusterEventRule(rule.Identifier) && isOrphanOfDeletedCluster(rule.CloudProviderResource, rule.ClusterId, liveClusterNames, now) {
			orphaned = append(orphaned, rule)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("orphaned CloudWatch event of a deleted cluster", len(orphaned), region)
	log.Info(count)

	if options.DryRun || len(orphaned) == 0 {
		return
	}

	log.Info(start)

	for _, rule := range orphaned {
		_ = removeTarget(*sessions.EventBridge, rule)
		if err := deleteCloudWatchEvent(*sessions.EventBridge, rule); err != nil {
			log.Errorf("Deletion CloudWatch event error %s/%s: %s", rule.Identifier, region, err)
		} else {
			log.Debugf("CloudWatch event %s of deleted cluster %s in %s deleted.", rule.Identifier, rule.ClusterId, region)
		}
	}
}

func DeleteOrphanedSQSQueues(sessions AWSSessions, options AwsOptions) {
	region := *sessions.SQS.Config.Region
	liveClusterNames, ok := liveClusterNamesForSweep(sessions, options, region)
	if !ok {
		return
	}

	queues, err := listTaggedSqsQueues(*sessions.SQS, options.TagName)
	if err != nil {
		log.Errorf("Can't list SQS queues in %s: %s", region, err)
		return
	}

	now := time.Now().UTC()
	var orphaned []sqsQueue
	for _, queue := range queues {
		if isQoveryClusterQueue(queue.Identifier, queue.ClusterId) && isOrphanOfDeletedCluster(queue.CloudProviderResource, queue.ClusterId, liveClusterNames, now) {
			orphaned = append(orphaned, queue)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("orphaned SQS queue of a deleted cluster", len(orphaned), region)
	log.Info(count)

	if options.DryRun || len(orphaned) == 0 {
		return
	}

	log.Info(start)

	for _, queue := range orphaned {
		if err := deleteSqsQueue(*sessions.SQS, queue); err != nil {
			log.Errorf("Deletion SQS queue error %s/%s: %s", queue.Identifier, region, err)
		} else {
			log.Debugf("SQS queue %s of deleted cluster %s in %s deleted.", queue.Identifier, queue.ClusterId, region)
		}
	}
}

type clusterSubnetGroup struct {
	common.CloudProviderResource
	ClusterId string
}

func listClusterSubnetGroups(svc *rds.RDS, tagName string) ([]clusterSubnetGroup, error) {
	var groups []clusterSubnetGroup
	err := svc.DescribeDBSubnetGroupsPages(&rds.DescribeDBSubnetGroupsInput{}, func(page *rds.DescribeDBSubnetGroupsOutput, _ bool) bool {
		for _, group := range page.DBSubnetGroups {
			tags, err := svc.ListTagsForResource(&rds.ListTagsForResourceInput{ResourceName: group.DBSubnetGroupArn})
			if err != nil {
				continue
			}

			essentialTags := common.GetEssentialTags(tags.TagList, tagName)
			groups = append(groups, clusterSubnetGroup{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   aws.StringValue(group.DBSubnetGroupName),
					Description:  "DB Subnet Group: " + aws.StringValue(group.DBSubnetGroupName),
					CreationDate: essentialTags.CreationDate,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				ClusterId: essentialTags.ClusterId,
			})
		}
		return true
	})

	return groups, err
}

// DocumentDB subnet groups included: same RDS API.
func DeleteOrphanedRDSSubnetGroups(sessions AWSSessions, options AwsOptions) {
	region := *sessions.RDS.Config.Region
	liveClusterNames, ok := liveClusterNamesForSweep(sessions, options, region)
	if !ok {
		return
	}

	groups, err := listClusterSubnetGroups(sessions.RDS, options.TagName)
	if err != nil {
		log.Errorf("Can't list RDS subnet groups in %s: %s", region, err)
		return
	}

	now := time.Now().UTC()
	var orphaned []clusterSubnetGroup
	for _, group := range groups {
		if isQoveryClusterSubnetGroup(group.Identifier) && isOrphanOfDeletedCluster(group.CloudProviderResource, group.ClusterId, liveClusterNames, now) {
			orphaned = append(orphaned, group)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("orphaned RDS subnet group of a deleted cluster", len(orphaned), region)
	log.Info(count)

	if options.DryRun || len(orphaned) == 0 {
		return
	}

	log.Info(start)

	// RDS refuses delete while a database uses the group; DeleteRDSSubnetGroup logs it.
	for _, group := range orphaned {
		DeleteRDSSubnetGroup(*sessions.RDS, group.Identifier)
	}
}
