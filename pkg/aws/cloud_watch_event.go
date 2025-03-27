package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	log "github.com/sirupsen/logrus"
)

type cloudWatchEvent struct {
	common.CloudProviderResource
}

func listTaggedCloudWatchEvents(svc eventbridge.EventBridge, tagName string) ([]cloudWatchEvent, error) {
	var taggedCloudwatchEvents []cloudWatchEvent

	MaxResultsPerPager := int64(100)

	params := &eventbridge.ListRulesInput{
		Limit: aws.Int64(MaxResultsPerPager), // Set the maximum number of results per page
	}

	for {
		result, err := svc.ListRules(params)
		if err != nil {
			return nil, err
		}

		if len(result.Rules) == 0 {
			return nil, nil
		}

		for _, rule := range result.Rules {
			tags, err := svc.ListTagsForResource(
				&eventbridge.ListTagsForResourceInput{
					ResourceARN: rule.Arn,
				},
			)
			if err != nil {
				continue
			}

			essentialTags := common.GetEssentialTags(tags.Tags, tagName)

			taggedCloudwatchEvents = append(taggedCloudwatchEvents, cloudWatchEvent{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   *rule.Name,
					Description:  "Cloudwatch event rule: " + *rule.Name,
					CreationDate: essentialTags.CreationDate,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
			})
		}
		if result.NextToken != nil {
			params.NextToken = result.NextToken
		} else {
			break // No more pages to retrieve
		}
	}

	return taggedCloudwatchEvents, nil
}

func getExpiredCloudWatchEvents(ECsession *eventbridge.EventBridge, options *AwsOptions) ([]cloudWatchEvent, string) {
	cloudwatchEvents, err := listTaggedCloudWatchEvents(*ECsession, options.TagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("Can't list Cloudwatch event in region %s: %s", region, err.Error())
	}

	var expiredCloudwatchEvents []cloudWatchEvent
	for _, cloudwatchEvent := range cloudwatchEvents {

		if cloudwatchEvent.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredCloudwatchEvents = append(expiredCloudwatchEvents, cloudwatchEvent)
		}
	}

	return expiredCloudwatchEvents, region
}

func deleteCloudWatchEvent(svc eventbridge.EventBridge, event cloudWatchEvent) error {
	force := true

	_, err := svc.DeleteRule(
		&eventbridge.DeleteRuleInput{
			Force: &force,
			Name:  aws.String(event.Identifier),
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func removeTarget(svc eventbridge.EventBridge, event cloudWatchEvent) error {
	targetsInput := &eventbridge.ListTargetsByRuleInput{
		Rule: aws.String(event.Identifier),
	}
	var targetIdsPtr []*string

	targetsOutput, err := svc.ListTargetsByRule(targetsInput)
	if err == nil && len(targetsOutput.Targets) > 0 {
		for _, target := range targetsOutput.Targets {
			targetIdsPtr = append(targetIdsPtr, target.Id)
		}

		removeInput := &eventbridge.RemoveTargetsInput{
			Rule:  aws.String(event.Identifier),
			Ids:   targetIdsPtr,
			Force: aws.Bool(true),
		}

		_, err = svc.RemoveTargets(removeInput)
		if err != nil {
			log.Errorf("Remove target error %s: %s", event.Identifier, err.Error())
		}
	}

	return nil
}

func DeleteExpiredCloudWatchEvents(sessions AWSSessions, options AwsOptions) {
	expiredCloudwatchEvents, region := getExpiredCloudWatchEvents(sessions.EventBridge, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired CloudWatch Events", len(expiredCloudwatchEvents), region)

	log.Info(count)

	//if options.DryRun || len(expiredCloudwatchEvents) == 0 {
	//	return
	//}

	log.Info(start)

	for _, cloudwatchEvent := range expiredCloudwatchEvents {
		log.Debugf("cloudwatchEvent %s in %s deleted.", cloudwatchEvent.Identifier, region)

		removeError := removeTarget(*sessions.EventBridge, cloudwatchEvent)
		if removeError != nil {
			log.Errorf("Remove target error %s/%s: %s", cloudwatchEvent.Identifier, region, removeError.Error())
		}

		deletionErr := deleteCloudWatchEvent(*sessions.EventBridge, cloudwatchEvent)
		if deletionErr != nil {
			log.Errorf("Deletion CloudWatch event error %s/%s: %s", cloudwatchEvent.Identifier, region, deletionErr.Error())
		} else {
			log.Debugf("CloudWatch event %s in %s deleted.", cloudwatchEvent.Identifier, region)
		}
	}
}
