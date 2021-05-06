package ec2

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"time"
)

type EBSVolume struct {
	VolumeId    string
	CreatedTime time.Time
	Status      string
	TTL         int64
	IsProtected bool
}

func TagVolumesFromEksClusterForDeletion(ec2Session ec2.EC2, tagKey string, clusterName string) error {
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
			},
		})
	if err != nil {
		return fmt.Errorf("Can't tag volumes for cluster %s in region %s: %s", clusterName, *ec2Session.Config.Region, err.Error())
	}

	return nil
}

func deleteVolumes(ec2Session ec2.EC2, VolumesList []EBSVolume) error {
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
			return err
		}
	}
	return nil
}

func listTaggedVolumes(ec2Session ec2.EC2, tagName string) ([]EBSVolume, error) {
	var taggedVolumes []EBSVolume

	input := &ec2.DescribeVolumesInput{
		//Filters: []*ec2.Filter{
		//	{
		//		Name: aws.String("tag:" + tagName),
		//	},
		//},
	}

	result, err := ec2Session.DescribeVolumes(input)
	if err != nil {
		return nil, err
	}

	if len(result.Volumes) == 0 {
		return nil, nil
	}

	for _, currentVolume := range result.Volumes {
		_, ttl, isProtected, _, _ := utils.GetEssentialTags(currentVolume.Tags, tagName)

		taggedVolumes = append(taggedVolumes, EBSVolume{
			VolumeId:    *currentVolume.VolumeId,
			CreatedTime: *currentVolume.CreateTime,
			Status:      *currentVolume.State,
			TTL:        ttl,
			IsProtected: isProtected,
		})
	}

	return taggedVolumes, nil
}

func DeleteExpiredVolumes(ec2Session ec2.EC2, tagName string, dryRun bool) {
	volumes, err := listTaggedVolumes(ec2Session, tagName)
	region := ec2Session.Config.Region
	if err != nil {
		log.Errorf("Can't list volumes: %s\n", err)
		return
	}

	var expiredVolumes []EBSVolume
	for _, volume := range volumes {
		if utils.CheckIfExpired(volume.CreatedTime, volume.TTL) && !volume.IsProtected {
			expiredVolumes = append(expiredVolumes, volume)
		}
	}

	count, start:= utils.ElemToDeleteFormattedInfos("expired EBS volume", len(expiredVolumes), *region)

	log.Debug(count)

	if dryRun || len(expiredVolumes) == 0 {
		return
	}

	log.Debug(start)
	for _, volume := range volumes {
		deletionErr := deleteVolumes(ec2Session, volumes)
			if deletionErr != nil {
				log.Errorf("Deletion EBS %s (%s) error: %s",
					volume.VolumeId, *ec2Session.Config.Region, err)
			}
	}
}