package aws

import (
	"fmt"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	log "github.com/sirupsen/logrus"
	"time"
)

type FargateProfile struct {
	ClusterName        string
	FargateProfileName string
	Status             string
	IsExpired          bool
}

func ListExpiredFargateProfiles(eksSession *eks.EKS, clusterName string, options *AwsOptions) []FargateProfile {
	var expiredFargateProfiles []FargateProfile

	var token *string
	for {
		req, resp := eksSession.ListFargateProfilesRequest(&eks.ListFargateProfilesInput{
			ClusterName: aws.String(clusterName),
			NextToken:   token,
		})

		err := req.Send()
		if err != nil {
			log.Errorf("Error listing Fargate profiles: %v", err)
		}
		token = resp.NextToken
		for _, fargateProfileName := range resp.FargateProfileNames {
			profile, err := getFargateProfile(eksSession, clusterName, *fargateProfileName, options)
			if err != nil {
				log.Errorf("Error describing Fargate profile: %v", err)
				continue
			}
			if profile.IsExpired {
				expiredFargateProfiles = append(expiredFargateProfiles, profile)
			}
		}

		if token == nil {
			break
		}
	}

	return expiredFargateProfiles
}

func DeleteFargateProfile(eksSession *eks.EKS, fargateProfile FargateProfile, options *AwsOptions) error {
	region := eksSession.Config.Region
	if options.DryRun {
		log.Infof("Dry run: skipping deletion of Fargate profile %s in region %s.", fargateProfile.FargateProfileName, *region)
		return nil
	}
	log.Infof("Starting deletion of EKS Fargate profile %s in region %s.", fargateProfile.FargateProfileName, *eksSession.Config.Region)

	_, err := eksSession.DeleteFargateProfile(
		&eks.DeleteFargateProfileInput{
			ClusterName:        aws.String(fargateProfile.ClusterName),
			FargateProfileName: aws.String(fargateProfile.FargateProfileName),
		})

	if err != nil {
		log.Errorf("Error deleting EKS Fargate profile %s/%s: %v",
			fargateProfile.FargateProfileName, *eksSession.Config.Region, err)
	}

	log.Debugf("Successfully deleted EKS Fargate profile %s in region %s.", fargateProfile.FargateProfileName, *eksSession.Config.Region)
	return nil
}

func getFargateProfile(eksSession *eks.EKS, clusterName string, profileName string, options *AwsOptions) (FargateProfile, error) {
	resp, err := eksSession.DescribeFargateProfile(&eks.DescribeFargateProfileInput{
		ClusterName:        aws.String(clusterName),
		FargateProfileName: aws.String(profileName),
	})
	if err != nil {
		log.Errorf("Error describing Fargate profile %s/%s: %v",
			profileName, *eksSession.Config.Region, err)
		return FargateProfile{}, err
	}

	creationTime, _ := time.Parse(time.RFC3339, resp.FargateProfile.CreatedAt.Format(time.RFC3339))
	tags := common.GetEssentialTags(resp.FargateProfile.Tags, options.TagName)

	profile := FargateProfile{
		ClusterName:        aws.StringValue(resp.FargateProfile.ClusterName),
		FargateProfileName: aws.StringValue(resp.FargateProfile.FargateProfileName),
		Status:             aws.StringValue(resp.FargateProfile.Status),
		IsExpired:          common.CheckIfExpired(creationTime, tags.TTL, fmt.Sprintf("EKS fargate profile: %s", aws.StringValue(resp.FargateProfile.FargateProfileName)), options.DisableTTLCheck),
	}
	return profile, nil
}
