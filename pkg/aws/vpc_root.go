package aws

import (
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type VpcInfo struct {
	common.CloudProviderResource
	SecurityGroups    []SecurityGroup
	NatGateways       []NatGateway
	InternetGateways  []InternetGateway
	Subnets           []Subnet
	RouteTables       []RouteTable
	ElasticIps        []ElasticIp
	NetworkInterfaces []NetworkInterface
	Status            string
}

func GetVpcsIdsByClusterNameTag(ec2Session ec2.EC2, clusterName string) []*string {
	result, err := ec2Session.DescribeVpcs(
		&ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("tag:ClusterName"),
					Values: []*string{aws.String(clusterName)},
				},
			},
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	var vpcsIds []*string
	for _, vpc := range result.Vpcs {
		vpcsIds = append(vpcsIds, vpc.VpcId)
	}

	return vpcsIds
}

func getVPCs(ec2Session *ec2.EC2, tagName string) []*ec2.Vpc {
	input := &ec2.DescribeVpcsInput{}

	result, err := ec2Session.DescribeVpcs(input)
	if err != nil {
		log.Error(err)
		return nil
	}

	if len(result.Vpcs) == 0 {
		return nil
	}

	return result.Vpcs
}

func GetAllVPCs(ec2Session *ec2.EC2) []*ec2.Vpc {
	result, err := ec2Session.DescribeVpcs(&ec2.DescribeVpcsInput{})
	if err != nil {
		log.Error(err)
		return nil
	}

	if len(result.Vpcs) == 0 {
		return nil
	}

	return result.Vpcs
}

func listTaggedVPC(ec2Session *ec2.EC2, options *AwsOptions) ([]VpcInfo, error) {
	var taggedVPCs []VpcInfo
	var VPCs = getVPCs(ec2Session, options.TagName)

	for _, vpc := range VPCs {
		essentialTags := common.GetEssentialTags(vpc.Tags, options.TagName)
		taggedVpc := VpcInfo{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *vpc.VpcId,
				Description:  "VPC: " + *vpc.VpcId,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Status: *vpc.State,
		}

		if *vpc.State != "available" {
			continue
		}
		if len(vpc.Tags) == 0 {
			continue
		}

		for _, tag := range vpc.Tags {
			if *tag.Key == options.TagName {
				if *tag.Value == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Value)
					continue
				}

				taggedVpc.Tag = *tag.Value
			}

			getCompleteVpc(ec2Session, options, &taggedVpc)
		}

		if taggedVpc.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			taggedVPCs = append(taggedVPCs, taggedVpc)
		}

		// NOTE: this piece of code is meant to enable resources cleaning for some resources on cluster that should not be deleted.
		// Doing this will make sure we never get quota issues on cluster we don't delete.
		if options.TagName == "do_not_delete" && common.CheckIfExpired(taggedVpc.CreationDate, taggedVpc.TTL, taggedVpc.Description, options.DisableTTLCheck) {
			taggedVPCs = append(taggedVPCs, taggedVpc)
		}
	}

	return taggedVPCs, nil
}

func deleteVPC(sessions AWSSessions, options AwsOptions, VpcList []VpcInfo) error {
	if options.DryRun {
		return nil
	}

	if len(VpcList) == 0 {
		return nil
	}

	ec2Session := sessions.EC2
	region := ec2Session.Config.Region

	for _, vpc := range VpcList {
		if options.DisableTTLCheck {
			vpcId, isOk := os.LookupEnv("PROTECTED_VPC_ID")
			if !isOk || vpcId == "" {
				log.Fatalf("Unable to get PROTECTED_VPC_ID environment variable in order to protect VPC resources.")
			}
			if vpcId == vpc.Identifier {
				log.Debugf("Skipping VPC %s in region %s (protected vpc)", vpc.Identifier, *region)
				continue
			}
		}

		DeleteLoadBalancerByVpcId(sessions.ELB, vpc, options.DryRun)
		DeleteNetworkInterfacesByVpcId(ec2Session, vpc.Identifier)
		ReleaseElasticIps(ec2Session, vpc.ElasticIps)
		DeleteSecurityGroupsByIds(ec2Session, vpc.SecurityGroups)
		DeleteInternetGatewaysByIds(ec2Session, vpc.InternetGateways, vpc.Identifier)
		DeleteSubnetsByIds(ec2Session, vpc.Subnets)
		DeleteRouteTablesByIds(ec2Session, vpc.RouteTables)
		DeleteNatGatewaysByIds(ec2Session, vpc.NatGateways)

		_, deleteErr := ec2Session.DeleteVpc(
			&ec2.DeleteVpcInput{
				VpcId: aws.String(vpc.Identifier),
			},
		)
		if deleteErr != nil {
			log.Errorf("Can't delete VPC %s in %s yet: %s", vpc.Identifier, *region, deleteErr.Error())
		} else {
			log.Debugf("VPC %s in %s deleted.", vpc.Identifier, *ec2Session.Config.Region)
		}
	}

	return nil
}

func DeleteExpiredVPC(sessions AWSSessions, options AwsOptions) {
	VPCs, err := listTaggedVPC(sessions.EC2, &options)
	region := sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("can't list VPC: %s\n", err)
	}

	count, start := common.ElemToDeleteFormattedInfos("tagged VPC resource", len(VPCs), *region)

	log.Info(count)

	if options.DryRun || len(VPCs) == 0 {
		return
	}

	log.Info(start)

	_ = deleteVPC(sessions, options, VPCs)

}

func getCompleteVpc(ec2Session *ec2.EC2, options *AwsOptions, vpc *VpcInfo) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go SetSecurityGroupsIdsByVpcId(ec2Session, vpc, &waitGroup, options.TagName)
	waitGroup.Add(1)
	go SetNetworkInterfacesByVpcId(ec2Session, vpc, &waitGroup)
	waitGroup.Add(1)
	go SetElasticIpsByVpcId(ec2Session, vpc, &waitGroup, options.TagName)
	waitGroup.Add(1)
	go SetNatGatewaysIdsByVpcId(ec2Session, options, vpc, &waitGroup)
	waitGroup.Add(1)
	go SetInternetGatewaysIdsByVpcId(ec2Session, vpc, &waitGroup, options.TagName)
	waitGroup.Add(1)
	go SetSubnetsIdsByVpcId(ec2Session, vpc, &waitGroup, options.TagName)
	waitGroup.Add(1)
	go SetRouteTablesIdsByVpcId(ec2Session, vpc, &waitGroup, options.TagName)
	waitGroup.Add(1)
}
