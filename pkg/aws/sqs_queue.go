package aws

import (
	"strconv"
	"time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	log "github.com/sirupsen/logrus"
)

type sqsQueue struct {
	QueueUrl        string
	QueueCreateTime time.Time
	TTL             int64
	IsProtected     bool
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
		params := &sqs.GetQueueAttributesInput{
			QueueUrl:       queue,
			AttributeNames: aws.StringSlice([]string{"CreatedTimestamp"}),
		}
		attributes, _ := svc.GetQueueAttributes(params)
		createdTimestamp, err := strconv.ParseInt(*attributes.Attributes["CreatedTimestamp"], 10, 64)

		if err != nil {
			log.Error("Failed to get queue createdTimestamp: %s", *queue)
			continue
		}

		time, _ := time.Parse(time.RFC3339, time.Unix(createdTimestamp, 0).Format(time.RFC3339))
		taggedQueues = append(taggedQueues, sqsQueue{
			QueueUrl:        *queue,
			QueueCreateTime: time,
			TTL:             essentialTags.TTL,
			IsProtected:     essentialTags.IsProtected,
		})

	}

	return taggedQueues, nil
}

func deleteSqsQueue(svc sqs.SQS, queue sqsQueue) error {

	log.Infof("Deleting SQS queue %s in %s, expired after %d seconds",
		queue.QueueUrl, *svc.Config.Region, queue.TTL)

	_, err := svc.DeleteQueue(
		&sqs.DeleteQueueInput{
			QueueUrl: aws.String(queue.QueueUrl),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredQueues(ECsession *sqs.SQS, tagName string) ([]sqsQueue, string) {
	queues, err := listTaggedSqsQueues(*ECsession, tagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("can't list SQS Queues in region %s: %s", region, err.Error())
	}

	var expiredQueues []sqsQueue
	for _, queue := range queues {
		if common.CheckIfExpired(queue.QueueCreateTime, queue.TTL, "sqs: "+queue.QueueUrl) && !queue.IsProtected {
			expiredQueues = append(expiredQueues, queue)
		}
	}

	return expiredQueues, region
}

func DeleteExpiredSQSQueues(sessions AWSSessions, options AwsOptions) {
	expiredQueues, region := getExpiredQueues(sessions.SQS, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired SQS Queues", len(expiredQueues), region)

	log.Debug(count)

	if options.DryRun || len(expiredQueues) == 0 {
		return
	}

	log.Debug(start)

	for _, queue := range expiredQueues {
		deletionErr := deleteSqsQueue(*sessions.SQS, queue)
		if deletionErr != nil {
			log.Errorf("Deletion SQS queue error %s/%s: %s", queue.QueueUrl, region, deletionErr.Error())
		}
	}
}
