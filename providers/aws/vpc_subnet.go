package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type Subnet struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
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

func SetSubnetsIdsByVpcId (ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	var subnetsStruct []Subnet

	subnets := getSubnetsByVpcId(ec2Session, *vpc.VpcId)

	for _, subnet := range subnets {
		creationDate, ttl, isProtected, _, _ := utils.GetEssentialTags(subnet.Tags, tagName)

		var subnetStruct = Subnet{
			Id: *subnet.SubnetId,
			CreationDate: creationDate,
			ttl: ttl,
			IsProtected: isProtected,
		}
		subnetsStruct = append(subnetsStruct, subnetStruct)
	}

	vpc.Subnets = subnetsStruct
}

func DeleteSubnetsByIds (ec2Session ec2.EC2, subnets []Subnet) {
	for _, subnet := range subnets {
		if utils.CheckIfExpired(subnet.CreationDate, subnet.ttl, "vpc subnet: " + subnet.Id) && subnet.IsProtected {
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
