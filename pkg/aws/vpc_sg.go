package aws

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type SecurityGroup struct {
	Id                  string
	CreationDate        time.Time
	ttl                 int64
	IsProtected         bool
	IpPermissionIngress []*ec2.IpPermission
	IpPermissionEgress  []*ec2.IpPermission
}

func getSecurityGroupsByVpcId(ec2Session *ec2.EC2, vpcId string) []*ec2.SecurityGroup {
	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	result, err := ec2Session.DescribeSecurityGroups(input)
	if err != nil {
		log.Error(err)
	}

	return result.SecurityGroups
}

func SetSecurityGroupsIdsByVpcId(ec2Session *ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	var securityGroupsStruct []SecurityGroup

	securityGroups := getSecurityGroupsByVpcId(ec2Session, vpc.Identifier)

	for _, securityGroup := range securityGroups {
		if *securityGroup.GroupName != "default" {
			essentialTags := common.GetEssentialTags(securityGroup.Tags, tagName)
			var securityGroupStruct = SecurityGroup{
				Id:                  *securityGroup.GroupId,
				CreationDate:        essentialTags.CreationDate,
				ttl:                 essentialTags.TTL,
				IsProtected:         essentialTags.IsProtected,
				IpPermissionIngress: securityGroup.IpPermissions,
				IpPermissionEgress:  securityGroup.IpPermissionsEgress,
			}

			securityGroupsStruct = append(securityGroupsStruct, securityGroupStruct)
		}
	}

	vpc.SecurityGroups = securityGroupsStruct
}

func DeleteSecurityGroupsByIds(ec2Session *ec2.EC2, securityGroups []SecurityGroup) {
	for _, securityGroup := range securityGroups {
		if !securityGroup.IsProtected {
			deleteIpPermissions(ec2Session, securityGroup)

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

func deleteIpPermissions(ec2Session *ec2.EC2, securityGroup SecurityGroup) {
	if securityGroup.IpPermissionIngress != nil {
		_, ingressErr := ec2Session.RevokeSecurityGroupIngress(
			&ec2.RevokeSecurityGroupIngressInput{
				GroupId:       aws.String(securityGroup.Id),
				IpPermissions: securityGroup.IpPermissionIngress,
			})
		if ingressErr != nil {
			log.Error("Ingress Perms : " + ingressErr.Error())
		}
	}

	if securityGroup.IpPermissionEgress != nil {
		_, egressErr := ec2Session.RevokeSecurityGroupEgress(
			&ec2.RevokeSecurityGroupEgressInput{
				GroupId:       aws.String(securityGroup.Id),
				IpPermissions: securityGroup.IpPermissionEgress,
			})

		if egressErr != nil {
			log.Error("Egress Perms : " + egressErr.Error())
		}
	}
}
