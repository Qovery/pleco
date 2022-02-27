package aws

import (
	"time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	log "github.com/sirupsen/logrus"
)

type CloudformationStack struct {
	StackName 	string
	CreateTime   time.Time
	TTL          int64
	IsProtected  bool
}

func CloudformationSession(sess session.Session, region string) *cloudformation.CloudFormation {
	return cloudformation.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedStacks(svc cloudformation.CloudFormation, tagName string) ([]CloudformationStack, error) {
	var taggedStacks []CloudformationStack

	result, err := svc.ListStacks(nil)
	if err != nil {
		return nil, err
	}

	if len(result.StackSummaries) == 0 {
		return nil, nil
	}

	for _, stack := range result.StackSummaries {
		describeStacksInput := &cloudformation.DescribeStacksInput{
			StackName: aws.String(*stack.StackName),
		}

		stackDescriptionList, err := svc.DescribeStacks(describeStacksInput)
		
		if err != nil  {
			continue
		}
		stackDescription := stackDescriptionList.Stacks[0]

		essentialTags := common.GetEssentialTags(stackDescription.Tags, tagName)

		taggedStacks = append(taggedStacks, CloudformationStack{
			StackName: 	  *stack.StackName,
			CreateTime:   *stack.CreationTime,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})

	}

	return taggedStacks, nil
}

func deleteStack(svc cloudformation.CloudFormation, stack CloudformationStack) error {

	log.Infof("Deleting CloudFormation Stack %s in %s, expired after %d seconds",
				stack.StackName, *svc.Config.Region, stack.TTL)

	_, err := svc.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: &stack.StackName,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredStacks(ECsession *cloudformation.CloudFormation, tagName string) ([]CloudformationStack, string) {
	stacks, err := listTaggedStacks(*ECsession, tagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("can't list CloudFormation Stacks in region %s: %s", region, err.Error())
	}

	var expiredStacks []CloudformationStack
	for _, stack := range stacks {
		if common.CheckIfExpired(stack.CreateTime, stack.TTL, "cloudformation: "+stack.StackName) && !stack.IsProtected {
			expiredStacks = append(expiredStacks, stack)
		}
	}

	return expiredStacks, region
}

func DeleteExpiredStacks(sessions AWSSessions, options AwsOptions) {
	expiredStacks, region := getExpiredStacks(sessions.CloudFormation, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired CloudFormation Stacks", len(expiredStacks), region)

	log.Debug(count)

	if options.DryRun || len(expiredStacks) == 0 {
		return
	}

	log.Debug(start)

	for _, stack := range expiredStacks {
		deletionErr := deleteStack(*sessions.CloudFormation, stack)
		if deletionErr != nil {
			log.Errorf("Deletion CloudFormation Stack error %s/%s: %s", stack.StackName, region, deletionErr.Error())
		}
	}
}
