package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type CompleteLogGroup struct {
	common.CloudProviderResource
	clusterId string
}

func getCloudwatchLogs(svc *cloudwatchlogs.CloudWatchLogs) []*cloudwatchlogs.LogGroup {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		Limit: aws.Int64(50),
	}

	logs, err := svc.DescribeLogGroups(input)
	handleCloudwatchLogsError(err)

	return logs.LogGroups
}

func getCompleteLogGroup(svc *cloudwatchlogs.CloudWatchLogs, log cloudwatchlogs.LogGroup, tagName string) CompleteLogGroup {
	tags := getLogGroupTag(svc, *log.Arn)
	essentialTags := common.GetEssentialTags(tags, tagName)

	return CompleteLogGroup{
		CloudProviderResource: common.CloudProviderResource{
			Identifier:   *log.LogGroupName,
			Description:  "Log Group Name: " + *log.LogGroupName,
			CreationDate: time.Unix(*log.CreationTime/1000, 0),
			TTL:          essentialTags.TTL,
			Tag:          essentialTags.Tag,
			IsProtected:  essentialTags.IsProtected,
		},
		clusterId: essentialTags.ClusterId,
	}
}

func deleteCloudwatchLog(svc cloudwatchlogs.CloudWatchLogs, logGroupName string) (string, error) {
	input := &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	}

	result, err := svc.DeleteLogGroup(input)
	handleCloudwatchLogsError(err)

	return result.String(), err
}

func getLogGroupTag(svc *cloudwatchlogs.CloudWatchLogs, logGroupARN string) map[string]*string {
	input := &cloudwatchlogs.ListTagsForResourceInput{ResourceArn: aws.String(logGroupARN)}

	req, tags := svc.ListTagsForResourceRequest(input)
	handleCloudwatchLogsError(req.Error)

	return tags.Tags
}

func handleCloudwatchLogsError(err error) {
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudwatchlogs.ErrCodeInvalidOperationException:
				log.Error(cloudwatchlogs.ErrCodeInvalidOperationException, aerr.Error())
			case cloudwatchlogs.ErrCodeInvalidParameterException:
				log.Error(cloudwatchlogs.ErrCodeInvalidParameterException, aerr.Error())
			case cloudwatchlogs.ErrCodeInvalidSequenceTokenException:
				log.Error(cloudwatchlogs.ErrCodeInvalidSequenceTokenException, aerr.Error())
			case cloudwatchlogs.ErrCodeServiceUnavailableException:
				log.Error(cloudwatchlogs.ErrCodeServiceUnavailableException, aerr.Error())
			case cloudwatchlogs.ErrCodeUnrecognizedClientException:
				log.Error(cloudwatchlogs.ErrCodeUnrecognizedClientException, aerr.Error())
			default:
				log.Error(aerr.Error())
			}
		} else {
			log.Error(err.Error())
		}

	}
}

func DeleteExpiredLogs(sessions AWSSessions, options AwsOptions) {
	logs := getCloudwatchLogs(sessions.CloudWatchLogs)
	region := *sessions.CloudWatchLogs.Config.Region
	var expiredLogs []CompleteLogGroup
	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(sessions.CloudWatchLogs, *log, options.TagName)
		if completeLogGroup.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredLogs = append(expiredLogs, completeLogGroup)
		}
	}

	count, start := common.ElemToDeleteFormattedInfos("expired Cloudwatch log", len(expiredLogs), region)

	log.Info(count)

	if options.DryRun || len(expiredLogs) == 0 {
		return
	}

	log.Info(start)

	for _, completeLog := range expiredLogs {
		_, deletionErr := deleteCloudwatchLog(*sessions.CloudWatchLogs, completeLog.Identifier)
		if deletionErr != nil {
			log.Errorf("Deletion Cloudwatch error %s/%s: %s",
				completeLog.Identifier, region, deletionErr)
		} else {
			log.Debugf("Cloudwatch logs %s in %s deleted.", completeLog.Identifier, region)
		}
	}

}

func addTtlToLogGroup(svc *cloudwatchlogs.CloudWatchLogs, logGroupARN string, ttl int64) (string, error) {
	input := &cloudwatchlogs.TagResourceInput{
		ResourceArn: aws.String(logGroupARN),
		Tags:        aws.StringMap(map[string]string{"ttl": fmt.Sprintf("%d", ttl)}),
	}

	result, err := svc.TagResource(input)
	handleCloudwatchLogsError(err)

	return result.String(), err
}

func TagLogsForDeletion(svc *cloudwatchlogs.CloudWatchLogs, tagName string, clusterId string, TTL int64) error {
	logs := getCloudwatchLogs(svc)

	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(svc, *log, tagName)

		if completeLogGroup.TTL == 0 && strings.Contains(completeLogGroup.Identifier, clusterId) {
			_, err := addTtlToLogGroup(svc, completeLogGroup.Identifier, TTL)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteUnlinkedLogs(sessions AWSSessions, options AwsOptions) {
	region := *sessions.EKS.Config.Region
	clusters, err := ListClusters(*sessions.EKS)
	if err != nil {
		log.Errorf("Can't list cluster in region %s: %s", region, err.Error())
	}

	deletableLogs := getUnlinkedLogs(sessions.CloudWatchLogs, clusters)

	count, start := common.ElemToDeleteFormattedInfos("unlinked Cloudwatch log", len(deletableLogs), region)

	log.Info(count)

	if options.DryRun || len(deletableLogs) == 0 {
		return
	}

	log.Info(start)

	for _, deletableLog := range deletableLogs {
		if deletableLog != "null" {
			_, deletionErr := deleteCloudwatchLog(*sessions.CloudWatchLogs, deletableLog)
			if deletionErr != nil {
				log.Errorf("Deletion Cloudwatch error %s/%s: %s",
					deletableLog, region, deletionErr)
			}
		}
	}
}

func getUnlinkedLogs(svc *cloudwatchlogs.CloudWatchLogs, clusters []*string) []string {
	logs := getCloudwatchLogs(svc)
	deletableLogs := make(map[string]string)

	for _, cluster := range clusters {
		for _, logGroup := range logs {
			if strings.Contains(*logGroup.LogGroupName, "/aws/eks/") && deletableLogs[*logGroup.LogGroupName] != "null" {
				if !strings.Contains(*logGroup.LogGroupName, (*cluster)[strings.Index(*cluster, "-")+1:len(*cluster)]) {
					deletableLogs[*logGroup.LogGroupName] = *logGroup.LogGroupName
				} else {
					deletableLogs[*logGroup.LogGroupName] = "null"
				}
			}
		}
	}

	var unlinkedLogs []string
	for _, deletableLog := range deletableLogs {
		if deletableLog != "null" {
			unlinkedLogs = append(unlinkedLogs, deletableLog)
		}
	}

	return unlinkedLogs
}
