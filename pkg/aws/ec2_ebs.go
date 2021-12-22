package aws

import (
	"fmt"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type EBSVolume struct {
	VolumeId    string
	CreatedTime time.Time
	Status      string
	TTL         int64
	IsProtected bool
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
			log.Infof("Volume %s in region %s is already in deletion process, skipping...", volume.VolumeId, *ec2Session.Config.Region)
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
				VolumeId: &volume.VolumeId,
			},
		)
		if err != nil {
			log.Errorf("Can't delete EBS %s in %s", volume.VolumeId, *ec2Session.Config.Region)
		}
	}
}

func listExpiredVolumes(eksSession *eks.EKS, ec2Session *ec2.EC2, tagName string) ([]EBSVolume, error) {
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

		essentialTags := common.GetEssentialTags(currentVolume.Tags, tagName)
		volume := EBSVolume{
			VolumeId:    *currentVolume.VolumeId,
			CreatedTime: essentialTags.CreationDate,
			Status:      *currentVolume.State,
			TTL:         essentialTags.TTL,
			IsProtected: essentialTags.IsProtected,
		}

		if !volume.IsProtected && (!common.IsAssociatedToLivingCluster(currentVolume.Tags, eksSession) || common.CheckIfExpired(volume.CreatedTime, volume.TTL, "EBS volume: "+volume.VolumeId)) {
			expiredVolumes = append(expiredVolumes, volume)
		}
	}

	return expiredVolumes, nil
}

func DeleteExpiredVolumes(sessions AWSSessions, options AwsOptions) {
	expiredVolumes, err := listExpiredVolumes(sessions.EKS, sessions.EC2, options.TagName)
	region := *sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("Can't list volumes: %s\n", err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("expired EBS volume", len(expiredVolumes), region)

	log.Debug(count)

	if options.DryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Debug(start)

	deleteVolumes(*sessions.EC2, expiredVolumes)
}
