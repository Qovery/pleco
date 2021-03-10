package aws

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type EBSVolume struct {
	VolumeId string
	CreatedTime time.Time
	Status string
	TTL int64
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
		return err
	}

	if len(result.Volumes) == 0 {
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
		return err
	}

	return nil
}

func deleteVolumes(ec2Session ec2.EC2, VolumesList []EBSVolume, dryRun bool) error {
	if dryRun {
		return nil
	}

	if len(VolumesList) == 0 {
		return nil
	}

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
	region := *ec2Session.Config.Region

	log.Debugf("Listing all volumes in region %s", region)
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
		log.Debugf("No volumes were found in region %s", region)
		return nil, nil
	}

	for _, currentVolume := range result.Volumes {
		ttlValue := ""
		for _, currentTag := range currentVolume.Tags {
			if *currentTag.Key == tagName {
				ttlValue = *currentTag.Value
			}
		}

		if ttlValue == "" {
			continue
		}

		ttlInt, err := strconv.Atoi(ttlValue)
		if err != nil {
			log.Errorf("Bad %s value on volume %s (%s), can't use it, it should be a number", tagName, *currentVolume.VolumeId, *ec2Session.Config.Region)
			continue
		}

		taggedVolumes = append(taggedVolumes, EBSVolume{
			VolumeId:    *currentVolume.VolumeId,
			CreatedTime: *currentVolume.CreateTime,
			Status:      *currentVolume.State,
			TTL:         int64(ttlInt),
		})
	}

	return taggedVolumes, nil
}

func DeleteExpiredVolumes(ec2Session ec2.EC2, tagName string, dryRun bool) error {
	volumes, err := listTaggedVolumes(ec2Session, tagName)
	if err != nil {
		return fmt.Errorf("Can't list volumes: %s\n", err)
	}

	for _, volume := range volumes {
		if utils.CheckIfExpired(volume.CreatedTime, volume.TTL) {
			err := deleteVolumes(ec2Session, volumes, dryRun)
			if err != nil {
				log.Errorf("Deletion EBS %s (%s) error: %s",
					volume.VolumeId, *ec2Session.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("EBS %s in %s, has not yet expired",
				volume.VolumeId, *ec2Session.Config.Region)
		}
	}

	return nil
}