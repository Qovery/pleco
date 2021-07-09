package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
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
	creationDate, ttl, isprotected, clusterId, tag := utils.GetEssentialTags(tags, tagName)

	return CompleteLogGroup{
		logGroupName: *log.LogGroupName,
		creationDate: creationDate,
		ttl:          ttl,
		clusterId:    clusterId,
		IsProtected:  isprotected,
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

func addTtlToLogGroup(svc cloudwatchlogs.CloudWatchLogs, logGroupName string) (string, error) {
	input := &cloudwatchlogs.TagLogGroupInput{
		LogGroupName: aws.String(logGroupName),
		Tags:         aws.StringMap(map[string]string{"ttl": "1"}),
	}

	result, err := svc.TagLogGroup(input)
	handleCloudwatchLogsError(err)

	return result.String(), err
}

func TagLogsForDeletion(svc cloudwatchlogs.CloudWatchLogs, tagName string, clusterId string) error {
	logs := getCloudwatchLogs(svc)
	var numberOfLogsToTag int64

	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(svc, *log, tagName)

		if completeLogGroup.ttl == 0 && strings.Contains(completeLogGroup.logGroupName, clusterId) {
			_, err := addTtlToLogGroup(svc, completeLogGroup.logGroupName)
			if err != nil {
				return err
			}
			numberOfLogsToTag++
		}
	}

	return nil
}
