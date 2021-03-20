package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"time"
)

type VpcInfo struct {
	VpcId            *string
	SecurityGroups   []SecurityGroup
	InternetGateways []InternetGateway
	Subnets          []Subnet
	RouteTables      []RouteTable
	Status           string
	TTL              int64
	Tag              string
}

func GetVpcsIdsByClusterNameTag(ec2Session ec2.EC2, clusterName string) []*string {
	result, err := ec2Session.DescribeVpcs(
		&ec2.DescribeVpcsInput{
			Filters: []*ec2.Filter{
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
	log.Debugf("Listing all VPCs")
	input := &ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag-key"),
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
		log.Debug("No VPCs were found")
		return nil
	}

	return result.Vpcs
}

func listTaggedVPC(ec2Session ec2.EC2, tagName string) ([]VpcInfo, error) {
	var taggedVPCs []VpcInfo

	var VPCs = getVPCs(ec2Session, tagName)

	for _, vpc := range VPCs {
		taggedVpc := VpcInfo{
			VpcId:  vpc.VpcId,
			Status: *vpc.State,
		}

		if *vpc.State != "available" {
			continue
		}
		if len(vpc.Tags) == 0 {
			continue
		}

		for _, tag := range vpc.Tags {
			if strings.EqualFold(*tag.Key, "ttl") {
				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on VPC %s in %v",
						*tag.Value, *vpc.VpcId, ec2Session.Config.Region)
				} else {
					taggedVpc.TTL = int64(ttl)
				}
			}

			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Key)
					continue
				}

				taggedVpc.Tag = *tag.Value
			}

			getCompleteVpc(ec2Session, &taggedVpc)

			taggedVPCs = append(taggedVPCs, taggedVpc)
		}
	}
	log.Debugf("Found %d VPC cluster(s) in ready status with ttl tag", len(taggedVPCs))

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
		DeleteSecurityGroupsByIds(ec2Session, vpc.SecurityGroups)
		DeleteInternetGatewaysByIds(ec2Session, vpc.InternetGateways)
		DeleteSubnetsByIds(ec2Session, vpc.Subnets)
		DeleteRouteTablesByIds(ec2Session, vpc.RouteTables)

		_, err := ec2Session.DeleteVpc(
			&ec2.DeleteVpcInput{
				VpcId: aws.String(*vpc.VpcId),
			},
		)
		if err != nil {
			// ignore errors, certainly due to dependencies that are not yet removed
			log.Warnf("Can't delete VPC %s in %s yet: %s", *vpc.VpcId, region, err.Error())
		}
	}

	_, err := ec2Session.DeleteVpc(
		&ec2.DeleteVpcInput{
			VpcId: aws.String(*VpcList[0].VpcId),
		},
	)
	if err != nil {
		// ignore errors, certainly due to dependencies that are not yet removed
		log.Warnf("Can't delete VPC %s in %s yet: %s", *VpcList[1].VpcId, region, err.Error())
	}

	return nil
}

func DeleteExpiredVPC(ec2Session ec2.EC2, tagName string, dryRun bool) error {
	VPCs, err := listTaggedVPC(ec2Session, tagName)

	if err != nil {
		return fmt.Errorf("can't list VPC: %s\n", err)
	}

	_ = deleteVPC(ec2Session, VPCs, dryRun)

	return nil
}

func getCompleteVpc(ec2Session ec2.EC2, vpc *VpcInfo) {
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

func TagVPCsForDeletion(ec2Session ec2.EC2, tagName string, clusterId string, clusterCreationTime time.Time, clusterTtl int64) error {
	vpcsIds := GetVpcsIdsByClusterNameTag(ec2Session, clusterId)

	err := AddCreationDateTagToSG(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return err
	}

	err = AddCreationDateTagToIGW(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return err
	}

	err = AddCreationDateTagToSubnets(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return err
	}

	err = AddCreationDateTagToRTB(ec2Session, vpcsIds, clusterCreationTime, clusterTtl)
	if err != nil {
		return err
	}

	return nil
}

// SUBNET

type Subnet struct {
	Id           string
	CreationDate time.Time
	ttl          int64
}

func getSubnetsByVpcId(ec2Session ec2.EC2, vpcId string) []*ec2.Subnet {
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
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

func getSubnetsByVpcsIds(ec2Session ec2.EC2, vpcsIds []*string) []*ec2.Subnet {
	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: vpcsIds,
			},
		},
	}

	result, err := ec2Session.DescribeSubnets(input)
	if err != nil {
		log.Error(err)
	}

	return result.Subnets
}

func SetSubnetsIdsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var subnetsStruct []Subnet

	subnets := getSubnetsByVpcId(ec2Session, *vpc.VpcId)

	for _, subnet := range subnets {
		creationDate, ttl := GetTimeInfos(subnet.Tags)

		var subnetStruct = Subnet{
			Id:           *subnet.SubnetId,
			CreationDate: creationDate,
			ttl:          ttl,
		}
		subnetsStruct = append(subnetsStruct, subnetStruct)
	}

	vpc.Subnets = subnetsStruct
}

func DeleteSubnetsByIds(ec2Session ec2.EC2, subnets []Subnet) {
	for _, subnet := range subnets {
		if CheckIfExpired(subnet.CreationDate, subnet.ttl) {
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

func AddCreationDateTagToSubnets(ec2Session ec2.EC2, vpcsIds []*string, creationDate time.Time, ttl int64) error {
	subnets := getSubnetsByVpcsIds(ec2Session, vpcsIds)
	var subnetsIds []*string

	for _, subnet := range subnets {
		subnetsIds = append(subnetsIds, subnet.SubnetId)
	}

	return AddCreationDateTag(ec2Session, subnetsIds, creationDate, ttl)
}

// SECUTIRY GROUP

type SecurityGroup struct {
	Id           string
	CreationDate time.Time
	ttl          int64
}

func getSecurityGroupsByVpcId(ec2Session ec2.EC2, vpcId string) []*ec2.SecurityGroup {
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

func getSecurityGroupsByVpcsIds(ec2Session ec2.EC2, vpcsIds []*string) []*ec2.SecurityGroup {
	result, err := ec2Session.DescribeSecurityGroups(
		&ec2.DescribeSecurityGroupsInput{
			Filters: []*ec2.Filter{
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

func SetSecurityGroupsIdsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var securityGroupsStruct []SecurityGroup

	securityGroups := getSecurityGroupsByVpcId(ec2Session, *vpc.VpcId)

	for _, securityGroup := range securityGroups {
		if *securityGroup.GroupName != "default" {
			creationDate, ttl := GetTimeInfos(securityGroup.Tags)

			var securityGroupStruct = SecurityGroup{
				Id:           *securityGroup.GroupId,
				CreationDate: creationDate,
				ttl:          ttl,
			}

			securityGroupsStruct = append(securityGroupsStruct, securityGroupStruct)
		}
	}

	vpc.SecurityGroups = securityGroupsStruct
}

func DeleteSecurityGroupsByIds(ec2Session ec2.EC2, securityGroups []SecurityGroup) {
	for _, securityGroup := range securityGroups {
		if CheckIfExpired(securityGroup.CreationDate, securityGroup.ttl) {
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

func deleteIpPermissions(ec2Session ec2.EC2, securityGroupId string) {
	_, ingressErr := ec2Session.RevokeSecurityGroupIngress(
		&ec2.RevokeSecurityGroupIngressInput{
			GroupId:    aws.String(securityGroupId),
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

func AddCreationDateTagToSG(ec2Session ec2.EC2, vpcsId []*string, creationDate time.Time, ttl int64) error {
	securityGroups := getSecurityGroupsByVpcsIds(ec2Session, vpcsId)
	var securityGroupsIds []*string

	for _, securityGroup := range securityGroups {
		securityGroupsIds = append(securityGroupsIds, securityGroup.GroupId)
	}

	return AddCreationDateTag(ec2Session, securityGroupsIds, creationDate, ttl)
}

// ROUTE TABLE

type RouteTable struct {
	Id           string
	CreationDate time.Time
	ttl          int64
	Associations []*ec2.RouteTableAssociation
}

func getRouteTablesByVpcId(ec2Session ec2.EC2, vpcId string) []*ec2.RouteTable {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	}

	routeTables, err := ec2Session.DescribeRouteTables(input)
	if err != nil {
		log.Error(err)
	}

	return routeTables.RouteTables
}

func getRouteTablesByVpcsIds(ec2Session ec2.EC2, vpcsIds []*string) []*ec2.RouteTable {
	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: vpcsIds,
			},
		},
	}

	result, err := ec2Session.DescribeRouteTables(input)
	if err != nil {
		log.Error(err)
	}

	return result.RouteTables
}

func SetRouteTablesIdsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var routeTablesStruct []RouteTable

	routeTables := getRouteTablesByVpcId(ec2Session, *vpc.VpcId)

	for _, routeTable := range routeTables {
		creationDate, ttl := GetTimeInfos(routeTable.Tags)

		var routeTableStruct = RouteTable{
			Id:           *routeTable.RouteTableId,
			CreationDate: creationDate,
			ttl:          ttl,
			Associations: routeTable.Associations,
		}
		routeTablesStruct = append(routeTablesStruct, routeTableStruct)
	}

	vpc.RouteTables = routeTablesStruct
}

func DeleteRouteTablesByIds(ec2Session ec2.EC2, routeTables []RouteTable) {
	for _, routeTable := range routeTables {
		if CheckIfExpired(routeTable.CreationDate, routeTable.ttl) && !isMainRouteTable(routeTable) {
			_, err := ec2Session.DeleteRouteTable(
				&ec2.DeleteRouteTableInput{
					RouteTableId: aws.String(routeTable.Id),
				},
			)

			if err != nil {
				log.Error(err)
			}
		}
	}
}

func AddCreationDateTagToRTB(ec2Session ec2.EC2, vpcsIds []*string, creationDate time.Time, ttl int64) error {
	routeTables := getRouteTablesByVpcsIds(ec2Session, vpcsIds)
	var routeTablesIds []*string

	for _, routeTable := range routeTables {
		routeTablesIds = append(routeTablesIds, routeTable.RouteTableId)
	}

	return AddCreationDateTag(ec2Session, routeTablesIds, creationDate, ttl)
}

func isMainRouteTable(routeTable RouteTable) bool {
	for _, association := range routeTable.Associations {
		if *association.Main && routeTable.Id == *association.RouteTableId {
			return true
		}
	}

	return false
}

// GATEWAY

type InternetGateway struct {
	Id           string
	CreationDate time.Time
	ttl          int64
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

func getInternetGatewaysByVpcsIds(ec2Session ec2.EC2, vpcsIds []*string) []*ec2.InternetGateway {
	input := &ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.vpc-id"),
				Values: vpcsIds,
			},
		},
	}

	result, err := ec2Session.DescribeInternetGateways(input)
	if err != nil {
		log.Error(err)
	}

	return result.InternetGateways
}

func SetInternetGatewaysIdsByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var internetGateways []InternetGateway

	gateways := getInternetGatewaysByVpcId(ec2Session, *vpc.VpcId)

	for _, gateway := range gateways {
		creationDate, ttl := GetTimeInfos(gateway.Tags)

		var gatewayStruct = InternetGateway{
			Id:           *gateway.InternetGatewayId,
			CreationDate: creationDate,
			ttl:          ttl,
		}

		internetGateways = append(internetGateways, gatewayStruct)
	}

	vpc.InternetGateways = internetGateways
}

func DeleteInternetGatewaysByIds(ec2Session ec2.EC2, internetGateways []InternetGateway) {
	for _, internetGateway := range internetGateways {
		if CheckIfExpired(internetGateway.CreationDate, internetGateway.ttl) {
			_, err := ec2Session.DeleteInternetGateway(
				&ec2.DeleteInternetGatewayInput{
					InternetGatewayId: aws.String(internetGateway.Id),
				},
			)

			if err != nil {
				log.Error(err)
			}
		}

	}
}

func AddCreationDateTagToIGW(ec2Session ec2.EC2, vpcsId []*string, creationDate time.Time, ttl int64) error {
	gateways := getInternetGatewaysByVpcsIds(ec2Session, vpcsId)
	var gatewaysIds []*string

	for _, gateway := range gateways {
		gatewaysIds = append(gatewaysIds, gateway.InternetGatewayId)
	}

	return AddCreationDateTag(ec2Session, gatewaysIds, creationDate, ttl)
}
