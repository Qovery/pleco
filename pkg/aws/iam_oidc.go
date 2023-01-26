package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

type OpenIDConnectProvider struct {
	common.CloudProviderResource
	OpenIDConnectProviderName string
}

func getOpenIDConnectProviders(iamSession *iam.IAM, tagName string) []OpenIDConnectProvider {
	var openIDConnectProviders []OpenIDConnectProvider

	result, err := iamSession.ListOpenIDConnectProviders(&iam.ListOpenIDConnectProvidersInput{})

	if err != nil {
		log.Error(err)
	}

	for _, openIDConnectProvider := range result.OpenIDConnectProviderList {
		tagsResult, tagsErr := iamSession.ListOpenIDConnectProviderTags(&iam.ListOpenIDConnectProviderTagsInput{
			OpenIDConnectProviderArn: openIDConnectProvider.Arn,
		})

		if tagsErr != nil {
			log.Error(tagsErr)
			continue
		}

		essentialTags := common.GetEssentialTags(tagsResult.Tags, tagName)
		openIDConnectProviders = append(openIDConnectProviders, OpenIDConnectProvider{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *openIDConnectProvider.Arn,
				Description:  "IAM OpenId Connect Provider: " + openIDConnectProvider.String(),
				CreationDate: essentialTags.CreationDate.UTC(),
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			OpenIDConnectProviderName: openIDConnectProvider.String(),
		})

	}

	return openIDConnectProviders
}

func getExpiredOpenIDConnectProviders(iamSession *iam.IAM, options *AwsOptions) []OpenIDConnectProvider {
	openIDConnectProviders := getOpenIDConnectProviders(iamSession, options.TagName)

	var expiredOpenIDConnectProviders []OpenIDConnectProvider
	for _, openIDConnectProvider := range openIDConnectProviders {
		if openIDConnectProvider.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredOpenIDConnectProviders = append(expiredOpenIDConnectProviders, openIDConnectProvider)
		}
	}

	return expiredOpenIDConnectProviders
}

func DeleteExpiredOpenIDConnectProviders(sessions *AWSSessions, options *AwsOptions) {
	expiredOpenIDConnectProviders := getExpiredOpenIDConnectProviders(sessions.IAM, options)

	count, start := common.ElemToDeleteFormattedInfos("expired OpenId Connect provider", len(expiredOpenIDConnectProviders), "Global")

	log.Info(count)

	if options.DryRun || len(expiredOpenIDConnectProviders) == 0 {
		return
	}

	log.Info(start)

	for _, expiredOpenIDConnectProvider := range expiredOpenIDConnectProviders {
		_, err := sessions.IAM.DeleteOpenIDConnectProvider(
			&iam.DeleteOpenIDConnectProviderInput{
				OpenIDConnectProviderArn: &expiredOpenIDConnectProvider.Identifier,
			})

		if err != nil {
			log.Errorf("Can't delete OpenId Connect provider %s : %s", expiredOpenIDConnectProvider.OpenIDConnectProviderName, err.Error())
		} else {
			log.Debugf("OpenId Connect provider %s deleted.", expiredOpenIDConnectProvider.OpenIDConnectProviderName)
		}
	}
}
