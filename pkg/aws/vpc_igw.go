package aws

import (
	"github.com/Qovery/pleco/pkg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type InternetGateway struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
}

func getInternetGatewaysByVpcId(ec2Session ec2.EC2, vpcId string) []*ec2.InternetGateway {
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	gateways, err := ec2Session.DescribeInternetGateways(input)
	if err != nil {
		log.Error(err)
	}

	return gateways.InternetGateways
}

func SetInternetGatewaysIdsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	var internetGateways []InternetGateway

	gateways := getInternetGatewaysByVpcId(ec2Session, *vpc.VpcId)

	for _, gateway := range gateways {
		creationDate, ttl, isProtected, _, _ := pkg.GetEssentialTags(gateway.Tags, tagName)

		var gatewayStruct = InternetGateway{
			Id:           *gateway.InternetGatewayId,
			CreationDate: creationDate,
			ttl:          ttl,
			IsProtected:  isProtected,
		}

		internetGateways = append(internetGateways, gatewayStruct)
	}

	vpc.InternetGateways = internetGateways
}

func DeleteInternetGatewaysByIds(ec2Session ec2.EC2, internetGateways []InternetGateway, vpcId string) {
	for _, internetGateway := range internetGateways {
		if !internetGateway.IsProtected {

			_, detachErr := ec2Session.DetachInternetGateway(
				&ec2.DetachInternetGatewayInput{
					InternetGatewayId: aws.String(internetGateway.Id),
					VpcId:             aws.String(vpcId),
				})

			if detachErr != nil {
				log.Error(detachErr)
			}

			_, deleteErr := ec2Session.DeleteInternetGateway(
				&ec2.DeleteInternetGatewayInput{
					InternetGatewayId: aws.String(internetGateway.Id),
				},
			)

			if deleteErr != nil {
				log.Error(deleteErr)
			}
		}
	}
}
