package vpc

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Subnet struct {
	Id string
	CreationDate time.Time
	ttl int64
}

func getSubnetsByVpcId (ec2Session ec2.EC2, vpcId string) []*ec2.Subnet {
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
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

func getSubnetsByVpcsIds (ec2Session ec2.EC2, vpcsIds []*string) []*ec2.Subnet {
	input := &ec2.DescribeSubnetsInput{
		Filters:  []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: vpcsIds,
			},
		},
	}

	result , err := ec2Session.DescribeSubnets(input)
	if err != nil {
		log.Error(err)
	}

	return result.Subnets
}

func SetSubnetsIdsByVpcId (ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var subnetsStruct []Subnet

	subnets := getSubnetsByVpcId(ec2Session, *vpc.VpcId)

	for _, subnet := range subnets {
		creationDate, ttl := utils.GetTimeInfos(subnet.Tags)

		var subnetStruct = Subnet{
			Id: *subnet.SubnetId,
			CreationDate: creationDate,
			ttl: ttl,
		}
		subnetsStruct = append(subnetsStruct, subnetStruct)
	}

	vpc.Subnets = subnetsStruct
}

func DeleteSubnetsByIds (ec2Session ec2.EC2, subnets []Subnet) {
	for _, subnet := range subnets {
		if utils.CheckIfExpired(subnet.CreationDate, subnet.ttl) {
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

func AddCreationDateTagToSubnets (ec2Session ec2.EC2, vpcsIds []*string, creationDate time.Time, ttl int64) error {
	subnets := getSubnetsByVpcsIds(ec2Session, vpcsIds)
	var subnetsIds []*string

	for _, subnet := range subnets {
		subnetsIds = append(subnetsIds, subnet.SubnetId)
	}

	return utils.AddCreationDateTag(ec2Session, subnetsIds, creationDate,ttl)
}
