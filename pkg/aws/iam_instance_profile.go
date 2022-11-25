package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"time"
)

type InstanceProfile struct {
	common.CloudProviderResource
	InstanceProfileName string
	Roles               []*iam.Role
}

func getInstanceProfiles(iamSession *iam.IAM, tagName string) []InstanceProfile {
	var instanceProfiles []InstanceProfile

	var token *string
	for {
		result, err := iamSession.ListInstanceProfiles(&iam.ListInstanceProfilesInput{
			Marker: token,
		})

		if err != nil {
			log.Error(err)
		}

		token = result.Marker

		for _, instanceProfile := range result.InstanceProfiles {
			essentialTags := common.GetEssentialTags(instanceProfile.Tags, tagName)
			instanceProfiles = append(instanceProfiles, InstanceProfile{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   *instanceProfile.InstanceProfileId,
					Description:  "IAM instance profile: " + *instanceProfile.InstanceProfileName,
					CreationDate: instanceProfile.CreateDate.UTC(),
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
				InstanceProfileName: *instanceProfile.InstanceProfileName,
				Roles:               instanceProfile.Roles,
			})
		}

		if result.Marker == nil {
			break
		}
	}

	return instanceProfiles
}

func getExpiredInstanceProfiles(iamSession *iam.IAM, options *AwsOptions) []InstanceProfile {
	instanceProfiles := getInstanceProfiles(iamSession, options.TagName)

	var expiredInstanceProfiles []InstanceProfile
	for _, instanceProfile := range instanceProfiles {
		if (len(instanceProfile.Roles) == 0 && time.Now().UTC().After(instanceProfile.CreationDate.Add(4*time.Hour))) || instanceProfile.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredInstanceProfiles = append(expiredInstanceProfiles, instanceProfile)
		}
	}

	return expiredInstanceProfiles
}

func DeleteExpiredInstanceProfiles(sessions *AWSSessions, options *AwsOptions) {
	expiredInstanceProfiles := getExpiredInstanceProfiles(sessions.IAM, options)

	count, start := common.ElemToDeleteFormattedInfos("expired instance profile", len(expiredInstanceProfiles), "Global")

	log.Info(count)

	if options.DryRun || len(expiredInstanceProfiles) == 0 {
		return
	}

	log.Info(start)

	for _, expiredInstanceProfile := range expiredInstanceProfiles {
		_, err := sessions.IAM.DeleteInstanceProfile(
			&iam.DeleteInstanceProfileInput{
				InstanceProfileName: &expiredInstanceProfile.InstanceProfileName,
			})

		if err != nil {
			log.Errorf("Can't delete instace profile %s : %s", expiredInstanceProfile.InstanceProfileName, err.Error())
		} else {
			log.Debugf("Instance profile %s deleted.", expiredInstanceProfile.InstanceProfileName)
		}
	}
}
