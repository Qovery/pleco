package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type VpcInfo struct {
	VpcId            *string
	SecurityGroups   []SecurityGroup
	NatGateways		 []NatGateway
	InternetGateways []InternetGateway
	Subnets          []Subnet
	RouteTables      []RouteTable
	NatGateways      []NatGateway
	Status           string
	TTL              int64
	Tag              string
	CreationDate     time.Time
	IsProtected      bool
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

func getVPCs(ec2Session ec2.EC2, tagName string) []*ec2.Vpc {
	input := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
				Values: []*string{&tagName},
			},
		},
	}

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

func listTaggedVPC(ec2Session ec2.EC2, tagName string) ([]VpcInfo, error) {
	var taggedVPCs []VpcInfo
	var VPCs = getVPCs(ec2Session, tagName)

	for _, vpc := range VPCs {
		creationDate, ttl, isprotected, _, _ := common.GetEssentialTags(vpc.Tags, tagName)
		taggedVpc := VpcInfo{
			VpcId:        vpc.VpcId,
			Status:       *vpc.State,
			CreationDate: creationDate,
			TTL:          ttl,
			IsProtected:  isprotected,
		}

		if *vpc.State != "available" {
			continue
		}
		if len(vpc.Tags) == 0 {
			continue
		}

		for _, tag := range vpc.Tags {
			if *tag.Key == tagName {
				if *tag.Value == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Value)
					continue
				}

				taggedVpc.Tag = *tag.Value
			}

			getCompleteVpc(ec2Session, &taggedVpc, tagName)
		}

		if common.CheckIfExpired(taggedVpc.CreationDate, taggedVpc.TTL, "vpc: "+*taggedVpc.VpcId) && !taggedVpc.IsProtected {
			taggedVPCs = append(taggedVPCs, taggedVpc)
		}

	}

	return taggedVPCs, nil
}

func deleteVPC(ec2Session ec2.EC2, VpcList []VpcInfo, dryRun bool) error {
	if dryRun {
		return nil
	}

	if len(VpcList) == 0 {
		return nil
	}

	region := *ec2Session.Config.Region

	for _, vpc := range VpcList {
		DeleteSecurityGroupsByIds(ec2Session, vpc.SecurityGroups)
		DeleteNatGatewaysByIds(ec2Session, vpc.NatGateways)
		DeleteInternetGatewaysByIds(ec2Session, vpc.InternetGateways, *vpc.VpcId)
		DeleteSubnetsByIds(ec2Session, vpc.Subnets)
		DeleteRouteTablesByIds(ec2Session, vpc.RouteTables)
		DeleteNatGatewaysByIds(ec2Session, vpc.NatGateways)

		_, deleteErr := ec2Session.DeleteVpc(
			&ec2.DeleteVpcInput{
				VpcId: aws.String(*vpc.VpcId),
			},
		)
		if deleteErr != nil {
			log.Errorf("Can't delete VPC %s in %s yet: %s", *vpc.VpcId, region, deleteErr.Error())
		}
	}

	return nil
}

func DeleteExpiredVPC(sessions *AWSSessions, options *AwsOption) {
	VPCs, err := listTaggedVPC(*sessions.EC2, options.TagName)
	region := sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("can't list VPC: %s\n", err)
	}

	count, start := common.ElemToDeleteFormattedInfos("tagged VPC resource", len(VPCs), *region)

	log.Debug(count)

	if options.DryRun || len(VPCs) == 0 {
		return
	}

	log.Debug(start)

	_ = deleteVPC(*sessions.EC2, VPCs, options.DryRun)

}

func getCompleteVpc(ec2Session ec2.EC2, vpc *VpcInfo, tagName string) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go SetSecurityGroupsIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Add(1)
	go SetNatGatewaysIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Add(1)
	go SetInternetGatewaysIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Add(1)
	go SetSubnetsIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Add(1)
	go SetRouteTablesIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Add(1)
	go SetNatGatewaysIdsByVpcId(ec2Session, vpc, &waitGroup, tagName)
	waitGroup.Wait()
}
