package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/eks"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type CompleteLogGroup struct {
	logGroupName string
	tag          string
	ttl          int64
	creationDate time.Time
	clusterId    string
	IsProtected  bool
}

func getCloudwatchLogs(svc cloudwatchlogs.CloudWatchLogs) []*cloudwatchlogs.LogGroup {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		Limit: aws.Int64(50),
	}

	logs, err := svc.DescribeLogGroups(input)
	handleCloudwatchLogsError(err)

	return logs.LogGroups
}

func getCompleteLogGroup(svc cloudwatchlogs.CloudWatchLogs, log cloudwatchlogs.LogGroup, tagName string) CompleteLogGroup {
	tags := getLogGroupTag(svc, *log.LogGroupName)
	_, ttl, isProtected, clusterId, tag := utils.GetEssentialTags(tags, tagName)

	return CompleteLogGroup{
		logGroupName: *log.LogGroupName,
		creationDate: time.Unix(*log.CreationTime / 1000, 0 ),
		ttl:          ttl,
		clusterId:    clusterId,
		IsProtected:  isProtected,
		tag:          tag,
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

func getLogGroupTag(svc cloudwatchlogs.CloudWatchLogs, logGroupName string) map[string]*string {
	input := &cloudwatchlogs.ListTagsLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	}

	tags, err := svc.ListTagsLogGroup(input)
	handleCloudwatchLogsError(err)

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

func DeleteExpiredLogs(svc cloudwatchlogs.CloudWatchLogs, tagName string, dryRun bool) {
	logs := getCloudwatchLogs(svc)
	region := svc.Config.Region
	var expiredLogs []CompleteLogGroup
	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(svc, *log, tagName)
		if utils.CheckIfExpired(completeLogGroup.creationDate, completeLogGroup.ttl, "log group: "+completeLogGroup.logGroupName) && !completeLogGroup.IsProtected {
			expiredLogs = append(expiredLogs, completeLogGroup)
		}
	}

	count, start := utils.ElemToDeleteFormattedInfos("expired Cloudwatch log", len(expiredLogs), *region)

	log.Debug(count)

	if dryRun || len(expiredLogs) == 0 {
		return
	}

	log.Debug(start)

	for _, completeLog := range expiredLogs {
		_, deletionErr := deleteCloudwatchLog(svc, completeLog.logGroupName)
		if deletionErr != nil {
			log.Errorf("Deletion Cloudwatch error %s/%s: %s",
				completeLog.logGroupName, *svc.Config.Region, deletionErr)
		}
	}

}

func addTtlToLogGroup(svc cloudwatchlogs.CloudWatchLogs, logGroupName string, ttl int64) (string, error) {
	input := &cloudwatchlogs.TagLogGroupInput{
		LogGroupName: aws.String(logGroupName),
		Tags:         aws.StringMap(map[string]string{"ttl": string(ttl)}),
	}

	result, err := svc.TagLogGroup(input)
	handleCloudwatchLogsError(err)

	return result.String(), err
}

func TagLogsForDeletion(svc cloudwatchlogs.CloudWatchLogs, tagName string, clusterId string, ttl int64) error {
	logs := getCloudwatchLogs(svc)

	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(svc, *log, tagName)

		if completeLogGroup.ttl == 0 && strings.Contains(completeLogGroup.logGroupName, clusterId) {
			_, err := addTtlToLogGroup(svc, completeLogGroup.logGroupName, ttl)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteUnlinkedLogs(svc cloudwatchlogs.CloudWatchLogs, eks eks.EKS, dryRun bool) {
	region := *eks.Config.Region
	clusters, err := ListClusters(eks)
	if err != nil {
		log.Errorf("C'ant list cluster in region %s: %s", region, err.Error())
	}

	deletableLogs := getUnlinkedLogs(svc, clusters)

	count, start := utils.ElemToDeleteFormattedInfos("unliked Cloudwatch log", len(deletableLogs), region)

	log.Debug(count)

	if dryRun || len(deletableLogs) == 0 {
		return
	}

	log.Debug(start)

	for _, deletableLog := range deletableLogs {
		if deletableLog != "null" {
			_, deletionErr := deleteCloudwatchLog(svc, deletableLog)
			if deletionErr != nil {
				log.Errorf("Deletion Cloudwatch error %s/%s: %s",
					deletableLog, region, deletionErr)
			}
		}
	}
}

func getUnlinkedLogs(svc cloudwatchlogs.CloudWatchLogs, clusters []*string) []string {
	logs := getCloudwatchLogs(svc)
	deletableLogs := make(map[string]string)

	for _, cluster := range clusters {
		for _, logGroup := range logs {
			if strings.Contains(*logGroup.LogGroupName, "/aws/eks/") && deletableLogs[*logGroup.LogGroupName] != "null" {
				if !strings.Contains(*logGroup.LogGroupName, (*cluster)[strings.Index(*cluster, "-") + 1 : len(*cluster)])  {
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
