package aws

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type Subnet struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
}

func getSubnetsByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.Subnet {
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	subnets, err := ec2Session.DescribeSubnets(input)
	if err != nil {
		log.Error(err)
	}

	return subnets.Subnets
}

func SetSubnetsIdsByVpcId(ec2Session *ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	var subnetsStruct []Subnet

	subnets := getSubnetsByVpcId(ec2Session, vpc.Identifier)

	for _, subnet := range subnets {
		essentialTags := common.GetEssentialTags(subnet.Tags, tagName)

		var subnetStruct = Subnet{
			Id:           *subnet.SubnetId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		}
		subnetsStruct = append(subnetsStruct, subnetStruct)
	}

	vpc.Subnets = subnetsStruct
}

func DeleteSubnetsByIds(ec2Session *ec2.EC2, subnets []Subnet) {
	for _, subnet := range subnets {
		if !subnet.IsProtected {
			_, err := ec2Session.DeleteSubnet(
				&ec2.DeleteSubnetInput{
					SubnetId: aws.String(subnet.Id),
				},
			)

			if err != nil {
				log.Error(err)
			}
		}
	}
}

// DeleteVPCLinkedResourcesWithQuota is used to delete some resources linked to a vpc without deleting the vpc itself.
// This will avoid quota issues on some resources
func DeleteVPCLinkedResourcesWithQuota(sessions AWSSessions, options AwsOptions) {
	vpcs, err := listTaggedVPC(sessions.EC2, &options)
	if err != nil {
		log.Errorf("can't list VPC: %s\n", err)
		return
	}

	region := *sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("Can't list instances: %s\n", err)
		return
	}

	securityGroupCount := 0
	subnetCount := 0
	routeTableCount := 0
	for _, vpc := range vpcs {
		securityGroupCount += len(vpc.SecurityGroups)
		subnetCount += len(vpc.Subnets)
		routeTableCount += len(vpc.RouteTables)
	}

	sgCount, sgStart := common.ElemToDeleteFormattedInfos("expired VPC Security Group", securityGroupCount, region)
	sCount, sStart := common.ElemToDeleteFormattedInfos("expired VPC Subnet", subnetCount, region)
	rtCount, rtStart := common.ElemToDeleteFormattedInfos("expired VPC Route Table", routeTableCount, region)

	log.Debug(sgCount)
	log.Debug(sCount)
	log.Debug(rtCount)

	if options.DryRun || len(vpcs) == 0 {
		return
	}

	log.Debug(sgStart)
	log.Debug(sStart)
	log.Debug(rtStart)

	for _, vpc := range vpcs {
		DeleteSecurityGroupsByIds(sessions.EC2, vpc.SecurityGroups)
		DeleteSubnetsByIds(sessions.EC2, vpc.Subnets)
		DeleteRouteTablesByIds(sessions.EC2, vpc.RouteTables)
	}
}
