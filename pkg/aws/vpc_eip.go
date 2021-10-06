package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type ElasticIp struct {
	Id            string
	AssociationId string
	Ip            string
	CreationDate  time.Time
	ttl           int64
	IsProtected   bool
}

func getElasticIps(ec2Session *ec2.EC2, tagName string) []ElasticIp {
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
		log.Errorf("Can't list elastic IPs in region %s: %s", *ec2Session.Config.Region, err.Error())
	}

	return responseToStruct(elasticIps, tagName)
}

func getElasticIpByNetworkInterfaceId(ec2Session ec2.EC2, niId string, vpcId string, tagName string) []ElasticIp {
	input := &ec2.DescribeAddressesInput{
		// only supporting EIP attached to VPC
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("domain"),
				Values: []*string{aws.String("vpc")},
			},
			{
				Name:   aws.String("network-interface-id"),
				Values: []*string{&niId},
			},
		},
	}

	elasticIps, err := ec2Session.DescribeAddresses(input)
	if err != nil {
		log.Errorf("Can't list elastic IPs for VPC %s in region %s: %s", vpcId, *ec2Session.Config.Region, err.Error())
	}

	return responseToStruct(elasticIps, tagName)
}

func ReleaseElasticIps(ec2Session ec2.EC2, eips []ElasticIp) {
	for _, eip := range eips {
		_, detachErr := ec2Session.DisassociateAddress(
			&ec2.DisassociateAddressInput{
				AssociationId: aws.String(eip.AssociationId),
			})

		if detachErr != nil {
			log.Errorf("Can't release EIP %s: %s", eip.Id, detachErr.Error())
		}
	}

}

func deleteElasticIp(ec2Session *ec2.EC2, eip ElasticIp) {
	_, releaseErr := ec2Session.ReleaseAddress(
		&ec2.ReleaseAddressInput{
			AllocationId: aws.String(eip.Id),
		})

	if releaseErr != nil {
		log.Errorf("Can't release EIP %s: %s", eip.Id, releaseErr.Error())
	}
}

func getExpiredEIPs(ec2Session *ec2.EC2, tagName string) []ElasticIp {
	elasticIps := getElasticIps(ec2Session, tagName)

	expiredEips := []ElasticIp{}
	for _, eip := range elasticIps {
		if common.CheckIfExpired(eip.CreationDate, eip.ttl, "eip: "+eip.Id) && !eip.IsProtected {
			expiredEips = append(expiredEips, eip)
		}
	}

	return expiredEips
}

func DeleteExpiredElasticIps(sessions *AWSSessions, options *AwsOptions) {
	expiredEips := getExpiredEIPs(sessions.EC2, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired EIP", len(expiredEips), *sessions.EC2.Config.Region)

	log.Debug(count)

	if options.DryRun || len(expiredEips) == 0 {
		return
	}

	log.Debug(start)

	for _, elasticIp := range expiredEips {
		deleteElasticIp(sessions.EC2, elasticIp)
	}
}

func SetElasticIpsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	for _, ni := range vpc.NetworkInterfaces {
		vpc.ElasticIps = append(vpc.ElasticIps, getElasticIpByNetworkInterfaceId(ec2Session, ni.Id, *vpc.VpcId, tagName)...)
	}
}

func responseToStruct(result *ec2.DescribeAddressesOutput, tagName string) []ElasticIp {
	eips := []ElasticIp{}
	for _, key := range result.Addresses {
		if key.AssociationId != nil && key.PublicIp != nil {
			essentialTags := common.GetEssentialTags(key.Tags, tagName)
			eip := ElasticIp{
				Id:            *key.AllocationId,
				AssociationId: *key.AssociationId,
				Ip:            *key.PublicIp,
				CreationDate:  essentialTags.CreationDate,
				ttl:           essentialTags.TTL,
				IsProtected:   essentialTags.IsProtected,
			}

			eips = append(eips, eip)
		}

	}

	return eips
}
