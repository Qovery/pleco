package aws

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type InternetGateway struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	IsProtected  bool
}

func getInternetGatewaysByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.InternetGateway {
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

func GetInternetGatewaysIdsByVpcId(ec2Session *ec2.EC2, vpcId string, tagName string) []InternetGateway {
	var internetGateways []InternetGateway

	gateways := getInternetGatewaysByVpcId(ec2Session, vpcId)

	for _, gateway := range gateways {
		essentialTags := common.GetEssentialTags(gateway.Tags, tagName)

		var gatewayStruct = InternetGateway{
			Id:           *gateway.InternetGatewayId,
			CreationDate: essentialTags.CreationDate,
			ttl:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		}

		internetGateways = append(internetGateways, gatewayStruct)
	}

	return internetGateways
}

func DeleteInternetGatewaysByIds(ec2Session *ec2.EC2, internetGateways []InternetGateway, vpcId string) {
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
			} else {
				log.Debugf("Internet Gateway %s in %s deleted.", internetGateway.Id, *ec2Session.Config.Region)
			}
		}
	}
}
