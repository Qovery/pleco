package aws

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
)

type NatGateway struct {
	common.CloudProviderResource
}

func getNatGatewaysByVpcId(ec2Session *ec2.EC2, options *AwsOptions, vpcId string) []NatGateway {
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

	return gtwResponseToStruct(gateways.NatGateways, options.TagName)
}

func getNatGateways(ec2Session *ec2.EC2, tagName string) []NatGateway {
	gateways, err := ec2Session.DescribeNatGateways(&ec2.DescribeNatGatewaysInput{})
	if err != nil {
		log.Error(err)
	}

	return gtwResponseToStruct(gateways.NatGateways, tagName)
}

func getExpiredNatGateways(ec2Session *ec2.EC2, options *AwsOptions) []NatGateway {
	gateways := getNatGateways(ec2Session, options.TagName)

	expiredGtws := []NatGateway{}
	for _, gtw := range gateways {
		if gtw.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredGtws = append(expiredGtws, gtw)
		}
	}

	return expiredGtws
}

func SetNatGatewaysIdsByVpcId(ec2Session *ec2.EC2, options *AwsOptions, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	gateways := getNatGatewaysByVpcId(ec2Session, options, vpc.Identifier)

	vpc.NatGateways = gateways
}

func DeleteNatGatewaysByIds(ec2Session *ec2.EC2, natGateways []NatGateway) {
	for _, natGateway := range natGateways {
		if !natGateway.IsProtected {

			_, deleteErr := ec2Session.DeleteNatGateway(
				&ec2.DeleteNatGatewayInput{
					NatGatewayId: aws.String(natGateway.Identifier),
				},
			)

			if deleteErr != nil {
				log.Error(deleteErr)
			} else {
				log.Debugf("Nat Gateway %s in %s deleted.", natGateway.Identifier, *ec2Session.Config.Region)
			}
		}
	}
}

func gtwResponseToStruct(result []*ec2.NatGateway, tagName string) []NatGateway {
	gtws := []NatGateway{}
	for _, key := range result {
		essentialTags := common.GetEssentialTags(key.Tags, tagName)
		gtw := NatGateway{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *key.NatGatewayId,
				Description:  "Nat Gateway: " + *key.NatGatewayId,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
		}

		gtws = append(gtws, gtw)
	}

	return gtws
}

func DeleteExpiredNatGateways(sessions AWSSessions, options AwsOptions) {
	gtws := getExpiredNatGateways(sessions.EC2, &options)
	region := sessions.EC2.Config.Region

	count, start := common.ElemToDeleteFormattedInfos("expired Nat Gateway", len(gtws), *region)

	log.Info(count)

	if options.DryRun || len(gtws) == 0 {
		return
	}

	log.Info(start)

	DeleteNatGatewaysByIds(sessions.EC2, gtws)

}
