package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type EBSVolume struct {
	VolumeId    string
	CreatedTime time.Time
	Status      string
	TTL         int64
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
		if CheckIfExpired(volume.CreatedTime, volume.TTL) {
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

//------------
// elb

type ElasticLoadBalancer struct {
	Arn         string
	Name        string
	CreatedTime time.Time
	Status      string
	TTL         int64
}

func TagLoadBalancersForDeletion(lbSession elbv2.ELBV2, tagKey string, loadBalancersList []ElasticLoadBalancer) error {
	var lbArns []*string

	if len(loadBalancersList) == 0 {
		return nil
	}

	for _, lb := range loadBalancersList {
		lbArns = append(lbArns, aws.String(lb.Arn))
	}

	if len(lbArns) == 0 {
		return nil
	}

	_, err := lbSession.AddTags(
		&elbv2.AddTagsInput{
			ResourceArns: lbArns,
			Tags: []*elbv2.Tag{
				{
					Key:   aws.String(tagKey),
					Value: aws.String("1"),
				},
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func ListTaggedLoadBalancersWithKeyContains(lbSession elbv2.ELBV2, tagContains string) ([]ElasticLoadBalancer, error) {
	var taggedLoadBalancers []ElasticLoadBalancer

	allLoadBalancers, err := ListLoadBalancers(lbSession)
	if err != nil {
		return nil, fmt.Errorf("Error while getting loadbalancer list on region %s\n", *lbSession.Config.Region)
	}

	// get lb tags and identify if one belongs to
	for _, currentLb := range allLoadBalancers {
		input := elbv2.DescribeTagsInput{ResourceArns: []*string{&currentLb.Arn}}

		result, err := lbSession.DescribeTags(&input)
		if err != nil {
			log.Errorf("Error while getting load balancer tags from %s", currentLb.Name)
			continue
		}

		for _, contentTag := range result.TagDescriptions[0].Tags {
			if strings.Contains(*contentTag.Key, tagContains) || strings.Contains(*contentTag.Value, tagContains) {
				taggedLoadBalancers = append(taggedLoadBalancers, currentLb)
			}
		}
	}

	return taggedLoadBalancers, nil
}

func listTaggedLoadBalancers(lbSession elbv2.ELBV2, tagName string) ([]ElasticLoadBalancer, error) {
	var taggedLoadBalancers []ElasticLoadBalancer
	region := *lbSession.Config.Region

	allLoadBalancers, err := ListLoadBalancers(lbSession)
	if err != nil {
		return nil, fmt.Errorf("Error while getting loadbalancer list on region %s\n", *lbSession.Config.Region)
	}

	if len(allLoadBalancers) == 0 {
		log.Debugf("No Load balancers were found in region %s", region)
		return nil, nil
	}

	// get tag with ttl
	for _, currentLb := range allLoadBalancers {
		input := elbv2.DescribeTagsInput{ResourceArns: []*string{&currentLb.Arn}}

		result, err := lbSession.DescribeTags(&input)
		if err != nil {
			log.Errorf("Error while getting load balancer tags from %s in %s", currentLb.Name, region)
			continue
		}

		for _, contentTag := range result.TagDescriptions[0].Tags {
			if *contentTag.Key == tagName {
				ttlInt, err := strconv.Atoi(*contentTag.Value)
				if err != nil {
					log.Errorf("Bad %s value on load balancer %s (%s), can't use it, it should be a number", tagName, currentLb.Name, region)
					continue
				}
				currentLb.TTL = int64(ttlInt)
				taggedLoadBalancers = append(taggedLoadBalancers, currentLb)
			}
		}
	}
	log.Debugf("Found %d Load balancers in ready status with ttl tag", len(taggedLoadBalancers))

	return taggedLoadBalancers, nil
}

func ListLoadBalancers(lbSession elbv2.ELBV2) ([]ElasticLoadBalancer, error) {
	var allLoadBalancers []ElasticLoadBalancer
	region := *lbSession.Config.Region

	log.Debugf("Listing all Loadbalancers in region %s", region)
	input := elbv2.DescribeLoadBalancersInput{}

	result, err := lbSession.DescribeLoadBalancers(&input)
	if err != nil {
		return nil, err
	}

	if len(result.LoadBalancers) == 0 {
		return nil, nil
	}

	for _, currentLb := range result.LoadBalancers {
		allLoadBalancers = append(allLoadBalancers, ElasticLoadBalancer{
			Arn:         *currentLb.LoadBalancerArn,
			Name:        *currentLb.LoadBalancerName,
			CreatedTime: *currentLb.CreatedTime,
			Status:      *currentLb.State.Code,
			TTL:         int64(-1),
		})
	}

	return allLoadBalancers, nil
}

func deleteLoadBalancers(lbSession elbv2.ELBV2, loadBalancersList []ElasticLoadBalancer, dryRun bool) error {
	if dryRun {
		return nil
	}

	if len(loadBalancersList) == 0 {
		return nil
	}

	for _, lb := range loadBalancersList {
		log.Infof("Deleting ELB %s in %s, expired after %d seconds",
			lb.Name, *lbSession.Config.Region, lb.TTL)
		_, err := lbSession.DeleteLoadBalancer(
			&elbv2.DeleteLoadBalancerInput{LoadBalancerArn: &lb.Arn},
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeleteExpiredLoadBalancers(elbSession elbv2.ELBV2, tagName string, dryRun bool) error {
	lbs, err := listTaggedLoadBalancers(elbSession, tagName)
	if err != nil {
		return fmt.Errorf("can't list Load Balancers: %s\n", err)
	}

	for _, lb := range lbs {
		if CheckIfExpired(lb.CreatedTime, lb.TTL) {
			err := deleteLoadBalancers(elbSession, lbs, dryRun)
			if err != nil {
				log.Errorf("Deletion ELB %s (%s) error: %s",
					lb.Name, *elbSession.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("Load Balancer %s in %s, has not yet expired",
				lb.Name, *elbSession.Config.Region)
		}
	}

	return nil
}

type KeyPair struct {
	KeyName      string
	KeyId        string
	CreationDate time.Time
	Tag          string
	ttl          int64
}

func getSshKeys(ec2session *ec2.EC2, tagName string) []KeyPair {
	result, err := ec2session.DescribeKeyPairs(
		&ec2.DescribeKeyPairsInput{})

	if err != nil {
		log.Error(err)
		return nil
	}

	var keys []KeyPair
	for _, key := range result.KeyPairs {
		newKey := KeyPair{
			KeyName: *key.KeyName,
			KeyId:   *key.KeyPairId,
		}

		for _, tag := range key.Tags {
			if strings.EqualFold(*tag.Key, tagName) {
				newKey.Tag = *tag.Value
			}
			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, _ := strconv.Atoi(*tag.Value)
				newKey.ttl = int64(ttl)
			}
		}

		if newKey.ttl != 0 {
			keys = append(keys, newKey)
		}
	}

	return keys
}

func deleteEc2Key(ec2session *ec2.EC2, keyId string) error {
	_, err := ec2session.DeleteKeyPair(
		&ec2.DeleteKeyPairInput{
			KeyPairId: aws.String(keyId),
		})

	return err
}

func DeleteExpiredKeys(ec2session *ec2.EC2, tagName string, dryRun bool) error {
	keys := getSshKeys(ec2session, tagName)
	var expiredKeysCount int64

	for _, key := range keys {
		if CheckIfExpired(key.CreationDate, key.ttl) {
			expiredKeysCount++

			if !dryRun {
				err := deleteEc2Key(ec2session, key.KeyId)
				if err != nil {
					return err
				}
			}
		}
	}

	log.Infof("There is %d expired key(s) to delete", expiredKeysCount)

	return nil
}
