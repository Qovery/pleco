package aws

import (
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type SecurityGroup struct {
	Id                  string
	CreationDate        time.Time
	ttl                 int64
	IsProtected         bool
	IpPermissionIngress []*ec2.IpPermission
	IpPermissionEgress  []*ec2.IpPermission
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

func SetSecurityGroupsIdsByVpcId (ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup, tagName string) {
	defer waitGroup.Done()
	var securityGroupsStruct []SecurityGroup

	securityGroups := getSecurityGroupsByVpcId(ec2Session, *vpc.VpcId)

	for _, securityGroup := range securityGroups {
		if *securityGroup.GroupName != "default" {
			creationDate, ttl, isProtected, _, _ := utils.GetEssentialTags(securityGroup.Tags, tagName)
			var securityGroupStruct = SecurityGroup{
				Id: 					*securityGroup.GroupId,
				CreationDate: 			creationDate,
				ttl: 					ttl,
				IsProtected: 			isProtected,
				IpPermissionIngress: 	securityGroup.IpPermissions,
				IpPermissionEgress: 	securityGroup.IpPermissionsEgress,
			}

			securityGroupsStruct = append(securityGroupsStruct, securityGroupStruct)
		}
	}


	vpc.SecurityGroups = securityGroupsStruct
}

func DeleteSecurityGroupsByIds (ec2Session ec2.EC2, securityGroups []SecurityGroup) {
	for _, securityGroup := range securityGroups {
		if !securityGroup.IsProtected{
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

func deleteIpPermissions (ec2Session ec2.EC2, securityGroup SecurityGroup) {
	_, ingressErr := ec2Session.RevokeSecurityGroupIngress(
		&ec2.RevokeSecurityGroupIngressInput{
			GroupId: aws.String(securityGroup.Id),
			IpPermissions: securityGroup.IpPermissionIngress,
		})

	if ingressErr != nil {
		log.Error("Ingress Perms : " + ingressErr.Error())
	}

	_, egressErr := ec2Session.RevokeSecurityGroupEgress(
		&ec2.RevokeSecurityGroupEgressInput{
			GroupId: aws.String(securityGroup.Id),
			IpPermissions: securityGroup.IpPermissionEgress,
		})

	if egressErr != nil {
		log.Error("Egress Perms : " + egressErr.Error())
	}

}
