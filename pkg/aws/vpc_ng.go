package aws

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type NatGateway struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
}

func getNatGatewaysByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.NatGateway {
	input := &ec2.DescribeNatGatewaysInput{
		Filter: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	gateways, err := ec2Session.DescribeNatGateways(input)
	if err != nil {
		log.Error(err)
	}

	return gateways.NatGateways
}

func SetNatGatewaysIdsByVpcId(ec2Session *ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()

	gateways := getNatGatewaysByVpcId(ec2Session, vpc.Identifier)
	natGateways := []NatGateway{}
	for _, gateway := range gateways {
		essentialTags := common.GetEssentialTags(gateway.Tags, tagName)

		var gatewayStruct = NatGateway{
			Id:           *gateway.NatGatewayId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		}

		natGateways = append(natGateways, gatewayStruct)
	}

	vpc.NatGateways = natGateways
}

func DeleteNatGatewaysByIds(ec2Session *ec2.EC2, natGateways []NatGateway) {
	for _, natGateway := range natGateways {
		if !natGateway.IsProtected {

			_, deleteErr := ec2Session.DeleteNatGateway(
				&ec2.DeleteNatGatewayInput{
					NatGatewayId: aws.String(natGateway.Id),
				},
			)

			if deleteErr != nil {
				log.Error(deleteErr)
			}
		}
	}
}
