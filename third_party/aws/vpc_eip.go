package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"time"
)

type ElasticIp struct {
	Id           string
	Ip           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
}

func getElasticIps(ec2Session *ec2.EC2, tagName string) []ElasticIp {
	var eips []ElasticIp

	input := &ec2.DescribeAddressesInput{
		// only supporting EIP attached to VPC
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("domain"),
				Values: []*string{aws.String("vpc")},
			},
		},
	}

	elasticIps, err := ec2Session.DescribeAddresses(input)
	if err != nil {
		log.Error(err)
	}

	for _, key := range elasticIps.Addresses {
		creationTime, ttl, isProtected, _, _ := utils.GetEssentialTags(key.Tags, tagName)
		eip := ElasticIp{
			Id:           *key.AssociationId,
			Ip:           *key.PublicIp,
			CreationDate: creationTime,
			ttl:          ttl,
			IsProtected:  isProtected,
		}

		eips = append(eips, eip)
	}

	return eips
}

func releaseElasticIp(ec2Session *ec2.EC2, allocationId string) error {
	_, detachErr := ec2Session.DisassociateAddress(
		&ec2.DisassociateAddressInput{
			AssociationId: aws.String(allocationId),
		})

	if detachErr != nil {
		return detachErr
	}

	_, releaseErr := ec2Session.ReleaseAddress(
		&ec2.ReleaseAddressInput{
			AllocationId: aws.String(allocationId),
		})

	if releaseErr != nil {
		return releaseErr
	}

	return nil
}

func DeleteExpiredElasticIps(sessions *AWSSessions, options *AwsOption) {
	elasticIps := getElasticIps(sessions.EC2, options.TagName)

	var expiredEips []ElasticIp
	for _, eip := range elasticIps {
		if utils.CheckIfExpired(eip.CreationDate, eip.ttl, "eip: "+eip.Id) && !eip.IsProtected {
			expiredEips = append(expiredEips, eip)
		}
	}

	if options.DryRun || len(expiredEips) == 0 {
		return
	}

	for _, elasticIp := range expiredEips {
		if !elasticIp.IsProtected {
			releaseErr := releaseElasticIp(sessions.EC2, options.TagName)
			if releaseErr != nil {
				log.Errorf("Release EIP error %s/%s: %s", elasticIp.Ip, elasticIp.Id, releaseErr)
			}
		}
	}
}
