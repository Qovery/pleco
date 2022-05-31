package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"

	"github.com/Qovery/pleco/pkg/common"
)

type ElasticIp struct {
	common.CloudProviderResource
	AssociationId string
	Ip            string
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

func getElasticIpByNetworkInterfaceId(ec2Session *ec2.EC2, niId string, vpcId string, tagName string) []ElasticIp {
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

func ReleaseElasticIps(ec2Session *ec2.EC2, eips []ElasticIp) {
	for _, eip := range eips {
		_, detachErr := ec2Session.DisassociateAddress(
			&ec2.DisassociateAddressInput{
				AssociationId: aws.String(eip.AssociationId),
			})

		if detachErr != nil {
			log.Errorf("Can't release EIP %s: %s", eip.Identifier, detachErr.Error())
		}
	}

}

func deleteElasticIp(ec2Session *ec2.EC2, eip ElasticIp) {
	_, releaseErr := ec2Session.ReleaseAddress(
		&ec2.ReleaseAddressInput{
			AllocationId: aws.String(eip.Identifier),
		})

	if releaseErr != nil {
		log.Errorf("Can't release EIP %s: %s", eip.Identifier, releaseErr.Error())
	}
}

func getExpiredEIPs(ec2Session *ec2.EC2, options *AwsOptions) []ElasticIp {
	elasticIps := getElasticIps(ec2Session, options.TagName)

	expiredEips := []ElasticIp{}
	for _, eip := range elasticIps {

		if eip.IsResourceExpired(options.TagValue) {
			expiredEips = append(expiredEips, eip)
		}
	}

	return expiredEips
}

func DeleteExpiredElasticIps(sessions AWSSessions, options AwsOptions) {
	expiredEips := getExpiredEIPs(sessions.EC2, &options)

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

func SetElasticIpsByVpcId(ec2Session *ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	for _, ni := range vpc.NetworkInterfaces {
		vpc.ElasticIps = append(vpc.ElasticIps, getElasticIpByNetworkInterfaceId(ec2Session, ni.Id, vpc.Identifier, tagName)...)
	}
}

func responseToStruct(result *ec2.DescribeAddressesOutput, tagName string) []ElasticIp {
	eips := []ElasticIp{}
	for _, key := range result.Addresses {
		essentialTags := common.GetEssentialTags(key.Tags, tagName)
		eip := ElasticIp{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *key.AllocationId,
				Description:  "Elastic IP: " + *key.AllocationId,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			AssociationId: "",
			Ip:            "",
		}

		if key.AssociationId != nil && key.PublicIp != nil {
			eip.AssociationId = *key.AssociationId
			eip.Ip = *key.PublicIp
		}

		eips = append(eips, eip)
	}

	return eips
}
