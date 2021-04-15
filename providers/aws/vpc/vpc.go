package vpc

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)



type VpcInfo struct {
	VpcId *string
	SecurityGroups []SecurityGroup
	InternetGateways []InternetGateway
	Subnets []Subnet
	RouteTables []RouteTable
	Status string
	TTL int64
	Tag string
	CreationDate time.Time
}

func GetVpcsIdsByClusterNameTag (ec2Session ec2.EC2, clusterName string) []*string {
	result, err := ec2Session.DescribeVpcs(
		&ec2.DescribeVpcsInput{
			Filters:    []*ec2.Filter{
				{
					Name:   aws.String("tag:ClusterName"),
					Values: []*string{aws.String(clusterName)},
				},
			},
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	var vpcsIds []*string
	for _, vpc := range result.Vpcs {
		vpcsIds = append(vpcsIds, vpc.VpcId)
	}

	return vpcsIds
}

func getVPCs(ec2Session ec2.EC2, tagName string) []*ec2.Vpc {
	input := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag-key"),
				Values: []*string{&tagName},
			},
		},
	}

	result, err := ec2Session.DescribeVpcs(input)
	if err != nil {
		log.Error(err)
		return nil
	}

	if len(result.Vpcs) == 0 {
		return nil
	}

	return result.Vpcs
}

func listTaggedVPC(ec2Session ec2.EC2, tagName string) ([]VpcInfo, error) {
	var taggedVPCs []VpcInfo

	var VPCs = getVPCs(ec2Session, tagName)

	for _, vpc := range VPCs {
		creationDate, ttl := utils.GetTimeInfos(vpc.Tags)
		taggedVpc := VpcInfo{
			VpcId:      vpc.VpcId,
			Status:     *vpc.State,
			CreationDate: creationDate,
			TTL: ttl,
		}

		if *vpc.State != "available" {
			continue
		}
		if len(vpc.Tags) == 0 {
			continue
		}

		for _, tag := range vpc.Tags {
			if *tag.Key == tagName {
				if *tag.Value == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Value)
					continue
				}

				taggedVpc.Tag = *tag.Value
			}

			getCompleteVpc(ec2Session, &taggedVpc)

		}

		if taggedVpc.CreationDate != time.Date(0001, 01, 01, 00, 00, 00, 0000, time.UTC) && utils.CheckIfExpired(taggedVpc.CreationDate, taggedVpc.TTL){
			taggedVPCs = append(taggedVPCs, taggedVpc)
		}

	}

	return taggedVPCs, nil
}

func deleteVPC(ec2Session ec2.EC2, VpcList []VpcInfo, dryRun bool) error {
	if dryRun {
		return nil
	}

	if len(VpcList) == 0 {
		return nil
	}

	region := *ec2Session.Config.Region

	for _, vpc := range VpcList {
		DeleteSecurityGroupsByIds(ec2Session,vpc.SecurityGroups)
		DeleteInternetGatewaysByIds(ec2Session, vpc.InternetGateways)
		DeleteSubnetsByIds(ec2Session, vpc.Subnets)
		DeleteRouteTablesByIds(ec2Session, vpc.RouteTables)

		_, err := ec2Session.DeleteVpc(
			&ec2.DeleteVpcInput{
				VpcId:  aws.String(*vpc.VpcId),
			},
		)
		if err != nil {
			// ignore errors, certainly due to dependencies that are not yet removed
			log.Warnf("Can't delete VPC %s in %s yet: %s", *vpc.VpcId, region, err.Error())
		}
	}

	_, err := ec2Session.DeleteVpc(
		&ec2.DeleteVpcInput{
			VpcId:  aws.String(*VpcList[0].VpcId),
		},
	)
	if err != nil {
		// ignore errors, certainly due to dependencies that are not yet removed
		log.Warnf("Can't delete VPC %s in %s yet: %s", *VpcList[1].VpcId, region, err.Error())
	}

	return nil
}

func DeleteExpiredVPC(ec2Session ec2.EC2, tagName string, dryRun bool) {
	VPCs, err := listTaggedVPC(ec2Session, tagName)
	region := ec2Session.Config.Region
	if err != nil {
		log.Errorf("can't list VPC: %s\n", err)
	}

	count, start := utils.ElemToDeleteFormattedInfos("tagged VPC resource", len(VPCs), *region)

	log.Debug(count)

	if dryRun || len(VPCs) == 0 {
		return
	}

	log.Debug(start)

	_ = deleteVPC(ec2Session, VPCs, dryRun)

}

func getCompleteVpc(ec2Session ec2.EC2, vpc *VpcInfo){
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go SetSecurityGroupsIdsByVpcId(ec2Session, vpc, &waitGroup)
	waitGroup.Add(1)
	go SetInternetGatewaysIdsByVpcId(ec2Session, vpc, &waitGroup)
	waitGroup.Add(1)
	go SetSubnetsIdsByVpcId(ec2Session, vpc, &waitGroup)
	waitGroup.Add(1)
	go SetRouteTablesIdsByVpcId(ec2Session, vpc, &waitGroup)
	waitGroup.Wait()
}

func TagVPCsForDeletion(ec2Session ec2.EC2, clusterId string, clusterCreationTime time.Time, clusterTtl int64) error {
	vpcsIds := GetVpcsIdsByClusterNameTag(ec2Session, clusterId)

	err := AddCreationDateTagToSG(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return fmt.Errorf("Can't tag security groups for cluster %s in region %s: %s", clusterId, * ec2Session.Config.Region, err.Error())
	}

 	err = AddCreationDateTagToIGW(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return fmt.Errorf("Can't tag internet gateways for cluster %s in region %s: %s", clusterId, * ec2Session.Config.Region, err.Error())
	}

	err = AddCreationDateTagToSubnets(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return fmt.Errorf("Can't tag subnets for cluster %s in region %s: %s", clusterId, * ec2Session.Config.Region, err.Error())
	}

	err = AddCreationDateTagToRTB(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return fmt.Errorf("Can't tag route tables for cluster %s in region %s: %s", clusterId, * ec2Session.Config.Region, err.Error())
	}

	return nil
}
