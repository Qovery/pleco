package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchevents"
	log "github.com/sirupsen/logrus"
)

type cloudWatchEvent struct {
	common.CloudProviderResource
}

func listTaggedCloudWatchEvents(svc cloudwatchevents.CloudWatchEvents, tagName string) ([]cloudWatchEvent, error) {
	var taggedCloudwatchEvents []cloudWatchEvent

	MaxResultsPerPager := int64(1000)

	params := &cloudwatchevents.ListRulesInput{
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
				&cloudwatchevents.ListTagsForResourceInput{
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
			log.Debug(*rule)
		}
		if result.NextToken != nil {
			params.NextToken = result.NextToken
		} else {
			break // No more pages to retrieve
		}
	}

	return taggedCloudwatchEvents, nil
}

func getExpiredCloudWatchEvents(ECsession *cloudwatchevents.CloudWatchEvents, options *AwsOptions) ([]cloudWatchEvent, string) {
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

func DeleteExpiredCloudWatchEvents(sessions AWSSessions, options AwsOptions) {
	expiredCloudwatchEvents, region := getExpiredCloudWatchEvents(sessions.CloudWatchEvents, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired CloudWatch Events", len(expiredCloudwatchEvents), region)

	log.Info(count)

	if options.DryRun || len(expiredCloudwatchEvents) == 0 {
		return
	}

	log.Info(start)

	for _, cloudwatchEvent := range expiredCloudwatchEvents {
		// TODO delete the events
		log.Debugf("cloudwatchEvent %s in %s deleted.", cloudwatchEvent.Identifier, region)
	}
}
