package vpc

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type SecurityGroup struct {
	Id string
	CreationDate time.Time
	ttl int64
}

func getSecurityGroupsByVpcId (ec2Session ec2.EC2, vpcId string) []*ec2.SecurityGroup {
	input := &ec2.DescribeSecurityGroupsInput{
		Filters:  []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	result , err := ec2Session.DescribeSecurityGroups(input)
	if err != nil {
		log.Error(err)
	}

	return result.SecurityGroups
}

func getSecurityGroupsByVpcsIds (ec2Session ec2.EC2, vpcsIds []*string) []*ec2.SecurityGroup{
		result , err := ec2Session.DescribeSecurityGroups(
			&ec2.DescribeSecurityGroupsInput{
				Filters:  []*ec2.Filter{
					{
						Name:   aws.String("vpc-id"),
						Values: vpcsIds,
					},
				},
			})
		if err != nil {
			log.Error(err)
		}

	return result.SecurityGroups
}

func SetSecurityGroupsIdsByVpcId (ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var securityGroupsStruct []SecurityGroup

	securityGroups := getSecurityGroupsByVpcId(ec2Session, *vpc.VpcId)

	for _, securityGroup := range securityGroups {
		if *securityGroup.GroupName != "default" {
			creationDate, ttl := utils.GetTimeInfos(securityGroup.Tags)

			var securityGroupStruct = SecurityGroup{
				Id: *securityGroup.GroupId,
				CreationDate: creationDate,
				ttl: ttl,
			}

			securityGroupsStruct = append(securityGroupsStruct, securityGroupStruct)
		}
	}


	vpc.SecurityGroups = securityGroupsStruct
}

func DeleteSecurityGroupsByIds (ec2Session ec2.EC2, securityGroups []SecurityGroup) {
	for _, securityGroup := range securityGroups {
		if utils.CheckIfExpired(securityGroup.CreationDate, securityGroup.ttl) {
			deleteIpPermissions(ec2Session, securityGroup.Id)

			_, err := ec2Session.DeleteSecurityGroup(
				&ec2.DeleteSecurityGroupInput{
					GroupId: aws.String(securityGroup.Id),
				},
			)

			if err != nil {
				log.Error(err)
			}
		}
	}
}

func deleteIpPermissions (ec2Session ec2.EC2, securityGroupId string) {
	_, ingressErr := ec2Session.RevokeSecurityGroupIngress(
		&ec2.RevokeSecurityGroupIngressInput{
			GroupId: aws.String(securityGroupId),
			IpProtocol: aws.String("-1"),
		})

	if ingressErr != nil {
		log.Warn("Ingress Perms : " + ingressErr.Error())
	}

	_, egressErr := ec2Session.RevokeSecurityGroupEgress(
		&ec2.RevokeSecurityGroupEgressInput{
			GroupId: aws.String(securityGroupId),
			IpPermissions: []*ec2.IpPermission{
				{
					IpProtocol: aws.String("-1"),
				},
			},
		})

	if egressErr != nil {
		log.Warn("Egress Perms : " + egressErr.Error())
	}

}

func AddCreationDateTagToSG (ec2Session ec2.EC2, vpcsId []*string, creationDate time.Time, ttl int64) error {
	securityGroups := getSecurityGroupsByVpcsIds(ec2Session, vpcsId)
	var securityGroupsIds []*string

	for _, securityGroup := range securityGroups {
		securityGroupsIds = append(securityGroupsIds, securityGroup.GroupId)
	}


	return utils.AddCreationDateTag(ec2Session, securityGroupsIds, creationDate, ttl)
}