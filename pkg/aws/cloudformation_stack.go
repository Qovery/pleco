package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type CloudformationStack struct {
	common.CloudProviderResource
}

func CloudformationSession(sess session.Session, region string) *cloudformation.CloudFormation {
	return cloudformation.New(&sess, &aws.Config{Region: aws.String(region)})
}

func tagStacks(svc cloudformation.CloudFormation, Stack *cloudformation.ListStacksOutput, tagName string) []CloudformationStack {
	var taggedStacks []CloudformationStack

	for _, stack := range Stack.StackSummaries {
		describeStacksInput := &cloudformation.DescribeStacksInput{
			StackName: aws.String(*stack.StackName),
		}

		stackDescriptionList, err := svc.DescribeStacks(describeStacksInput)

		if err != nil {
			continue
		}
		stackDescription := stackDescriptionList.Stacks[0]

		essentialTags := common.GetEssentialTags(stackDescription.Tags, tagName)

		taggedStacks = append(taggedStacks, CloudformationStack{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *stack.StackName,
				Description:  "Cloud Formation Stack: " + *stack.StackName,
				CreationDate: *stack.CreationTime,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
		})

	}
	return taggedStacks
}

func listTaggedStacks(svc cloudformation.CloudFormation, tagName string) ([]CloudformationStack, error) {

	result, err := svc.ListStacks(nil)
	if err != nil {
		return nil, err
	}

	if len(result.StackSummaries) == 0 {
		return nil, nil
	}

	taggedStacks := tagStacks(svc, result, tagName)

	for result.NextToken != nil {
		result, err = svc.ListStacks(&cloudformation.ListStacksInput{
			NextToken: result.NextToken,
		})

		if err != nil {
			return nil, err
		}
		taggedStacks = append(taggedStacks, tagStacks(svc, result, tagName)...)
	}

	return taggedStacks, nil
}

func deleteStack(svc cloudformation.CloudFormation, stack CloudformationStack) error {

	log.Infof("Deleting CloudFormation Stack %s in %s, expired after %d seconds",
		stack.Identifier, *svc.Config.Region, stack.TTL)

	_, err := svc.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: &stack.Identifier,
	},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredStacks(ECsession *cloudformation.CloudFormation, options *AwsOptions) ([]CloudformationStack, string) {
	stacks, err := listTaggedStacks(*ECsession, options.TagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("Can't list CloudFormation Stacks in region %s: %s", region, err.Error())
	}

	var expiredStacks []CloudformationStack
	for _, stack := range stacks {
		if stack.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredStacks = append(expiredStacks, stack)
		}
	}

	return expiredStacks, region
}

func DeleteExpiredStacks(sessions AWSSessions, options AwsOptions) {
	expiredStacks, region := getExpiredStacks(sessions.CloudFormation, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired CloudFormation Stacks", len(expiredStacks), region)

	log.Info(count)

	if options.DryRun || len(expiredStacks) == 0 {
		return
	}

	log.Info(start)

	for _, stack := range expiredStacks {
		deletionErr := deleteStack(*sessions.CloudFormation, stack)
		if deletionErr != nil {
			log.Errorf("Deletion CloudFormation Stack error %s/%s: %s", stack.Identifier, region, deletionErr.Error())
		} else {
			log.Debugf("CloudFormation Stack %s in %s deleted.", stack.Identifier, region)
		}
	}
}
