package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type EBSVolume struct {
	common.CloudProviderResource
	Status string
}

func TagVolumesFromEksClusterForDeletion(ec2Session *ec2.EC2, tagKey string, clusterName string) error {
	var volumesIds []*string

	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:kubernetes.io/cluster/" + clusterName),
				Values: []*string{
					aws.String("owned"),
				},
			},
		},
	}

	result, err := ec2Session.DescribeVolumes(input)
	if err != nil {
		return fmt.Errorf("Can't get volumes for cluster %s in region %s: %s", clusterName, *ec2Session.Config.Region, err.Error())
	}

	if len(result.Volumes) == 0 {
		log.Debugf("No volume to tag for cluster %s", clusterName)
		return nil
	}

	for _, currentVolume := range result.Volumes {
		volumesIds = append(volumesIds, currentVolume.VolumeId)
	}

	_, err = ec2Session.CreateTags(
		&ec2.CreateTagsInput{
			Resources: volumesIds,
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(tagKey),
					Value: aws.String("1"),
				},
				{
					Key:   aws.String("creationDate"),
					Value: aws.String(time.Now().Format(time.RFC3339)),
				},
			},
		})
	if err != nil {
		return fmt.Errorf("Can't tag volumes for cluster %s in region %s: %s", clusterName, *ec2Session.Config.Region, err.Error())
	}

	return nil
}

func deleteVolumes(ec2Session ec2.EC2, VolumesList []EBSVolume) {
	for _, volume := range VolumesList {
		switch volume.Status {
		case "deleting":
			log.Debugf("Volume %s in region %s is already in deletion process, skipping...", volume.Identifier, *ec2Session.Config.Region)
			continue
		case "creating":
			continue
		case "deleted":
			continue
		case "in-use":
			continue
		}

		_, err := ec2Session.DeleteVolume(
			&ec2.DeleteVolumeInput{
				VolumeId: &volume.Identifier,
			},
		)
		if err != nil {
			log.Errorf("Can't delete EBS %s in %s", volume.Identifier, *ec2Session.Config.Region)
		} else {
			log.Debugf("EBS %s in %s deleted.", volume.Identifier, *ec2Session.Config.Region)
		}
	}
}

func listExpiredVolumes(eksSession *eks.EKS, ec2Session *ec2.EC2, options *AwsOptions) ([]EBSVolume, error) {
	result, err := ec2Session.DescribeVolumes(&ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, err
	}

	if len(result.Volumes) == 0 {
		return nil, nil
	}

	var expiredVolumes []EBSVolume
	for _, currentVolume := range result.Volumes {
		if strings.Contains(*currentVolume.State, "in-use") {
			continue
		}

		essentialTags := common.GetEssentialTags(currentVolume.Tags, options.TagName)
		volume := EBSVolume{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *currentVolume.VolumeId,
				Description:  "EBS Volume: " + *currentVolume.VolumeId,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Status: *currentVolume.State,
		}

		if !volume.IsProtected && (!common.IsAssociatedToLivingCluster(currentVolume.Tags, eksSession) || volume.IsResourceExpired(options.TagValue, options.DisableTTLCheck)) {
			expiredVolumes = append(expiredVolumes, volume)
		}
	}

	return expiredVolumes, nil
}

func DeleteExpiredVolumes(sessions AWSSessions, options AwsOptions) {
	expiredVolumes, err := listExpiredVolumes(sessions.EKS, sessions.EC2, &options)
	region := *sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("Can't list volumes: %s\n", err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("expired EBS volume", len(expiredVolumes), region)

	log.Info(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Info(start)

	deleteVolumes(*sessions.EC2, expiredVolumes)
}
