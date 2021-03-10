package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type CompleteLogGroup struct {
	logGroupName string
	tag string
	ttl int64
	creationDate time.Time
	clusterId string
}

func getCloudwatchLogs(svc cloudwatchlogs.CloudWatchLogs)  []*cloudwatchlogs.LogGroup {
	input := &cloudwatchlogs.DescribeLogGroupsInput{
		Limit: aws.Int64(50),
	}

	logs, err := svc.DescribeLogGroups(input)
	handleCloudwatchLogsError(err)

	return logs.LogGroups
}

func getCompleteLogGroup(svc cloudwatchlogs.CloudWatchLogs, log cloudwatchlogs.LogGroup, tagName string) CompleteLogGroup {
	var completeLogGroup CompleteLogGroup
	tags := getLogGroupTag(svc, *log.LogGroupName)

	completeLogGroup.logGroupName = *log.LogGroupName
	completeLogGroup.creationDate = time.Unix(*log.CreationTime/1000,0)

	for key, element := range tags {
		if key == "ttl" {
			ttl , _ := strconv.ParseInt(*element,10,64)
			completeLogGroup.ttl = ttl
		}

		if key == "ClusterId" {
			completeLogGroup.clusterId = *element
		}

		if key == tagName {
			completeLogGroup.tag = *element
		}
	}

	return completeLogGroup
}

func deleteCloudwatchLog (svc cloudwatchlogs.CloudWatchLogs, logGroupName string) (string, error) {
	input := &cloudwatchlogs.DeleteLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	}

	result, err := svc.DeleteLogGroup(input)
	handleCloudwatchLogsError(err)

	return result.String(), err
}

func getLogGroupTag (svc cloudwatchlogs.CloudWatchLogs, logGroupName string) map[string]*string{
	input := &cloudwatchlogs.ListTagsLogGroupInput{
		LogGroupName: aws.String(logGroupName),
	}

	tags, err := svc.ListTagsLogGroup(input)
	handleCloudwatchLogsError(err)

	return tags.Tags
}

func handleCloudwatchLogsError(err error){
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

func DeleteExpiredLogs(svc cloudwatchlogs.CloudWatchLogs, tagName string, dryRun bool) error {
	logs := getCloudwatchLogs(svc)
	var numberOfLogsToDelete int64

	for _, log := range logs {
		completeLogGroup := getCompleteLogGroup(svc, *log, tagName)

		if completeLogGroup.ttl != 0 && utils.CheckIfExpired(completeLogGroup.creationDate, completeLogGroup.ttl) {
			if !dryRun {
				_, err := deleteCloudwatchLog(svc, completeLogGroup.logGroupName)
				if err != nil {
					return err
				}
			}

			numberOfLogsToDelete++
		}
	}

	log.Info("There is ", numberOfLogsToDelete, " expired log(s) to delete.")

	return nil
}
func addTtlToLogGroup(svc cloudwatchlogs.CloudWatchLogs, logGroupName string) (string,error) {
	input := &cloudwatchlogs.TagLogGroupInput{
		LogGroupName: aws.String(logGroupName),
		Tags: aws.StringMap(map[string]string{"ttl": "1" }),
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

		if completeLogGroup.ttl == 0 && strings.Contains(completeLogGroup.logGroupName, clusterId){
			_, err := addTtlToLogGroup(svc, completeLogGroup.logGroupName)
			if err != nil {
				return err
			}
			numberOfLogsToTag++
		}
	}

	log.Info("There is ", numberOfLogsToTag, " log(s) to tag with ttl.")

	return nil
}
