package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"strconv"
)

type vpcInfo struct {
	VpcId string
	Status string
	TTL int64
}

func listTaggedVPC(ec2Session ec2.EC2, tagName string) ([]vpcInfo, error) {
	var taggedVPC []vpcInfo

	log.Debugf("Listing all VPCs")
	input := &ec2.DescribeVpcsInput{
		Filters:    nil,
	}

	result, err := ec2Session.DescribeVpcs(input)
	if err != nil {
		return nil, err
	}

	if len(result.Vpcs) == 0 {
		log.Debug("No VPCs were found")
		return nil, nil
	}

	for _, vpc := range result.Vpcs {

		if *vpc.State != "available" {
			continue
		}
		if len(vpc.Tags) == 0 {
			continue
		}

		for _, tag := range vpc.Tags {
			if *tag.Key == tagName {
				if *tag.Key == "" {
					log.Warnf("Tag %s was empty and it wasn't expected, skipping", *tag.Key)
					continue
				}

				ttl, err := strconv.Atoi(*tag.Value)
				if err != nil {
					log.Errorf("Error while trying to convert tag value (%s) to integer on VPC %s in %v",
						*tag.Value, *vpc.VpcId, ec2Session.Config.Region)
					continue
				}

				taggedVPC = append(taggedVPC, vpcInfo{
					VpcId:      *vpc.VpcId,
					Status:     *vpc.State,
					TTL:        int64(ttl),
				})
			}
		}
	}
	log.Debugf("Found %d VPC cluster(s) in ready status with ttl tag", len(taggedVPC))

	return taggedVPC, nil
}

func deleteVPC(ec2Session ec2.EC2, VpcList []vpcInfo, dryRun bool) error {
	if dryRun {
		return nil
	}

	if len(VpcList) == 0 {
		return nil
	}

	// todo: delete security groups, subnets, internet gateways, routing tables
	/*
	 Validating resources to delete...
	 Detaching internet gateways...
	 Revoking security group rules...
	 Deleting VPC endpoints...
	 Deleting security groups...
	 Deleting egress only internet gateways...
	 Deleting internet gateways...
	 Deleting network interfaces...
	 Deleting subnets...
	 Deleting network ACLs...
	 Deleting route tables...
	 Deleting VPC...
	 Waiting for VPC endpoints to be deleted...
	 */
	region := *ec2Session.Config.Region

	for _, vpc := range VpcList {
		_, err := ec2Session.DeleteVpc(
			&ec2.DeleteVpcInput{
				VpcId:  &vpc.VpcId,
			},
		)
		if err != nil {
			// ignore errors, certainly due to dependencies that are not yet removed
			log.Warnf("Can't delete VPC %s in %s yet: %s", vpc.VpcId, region, err.Error())
		}
	}

	return nil
}

func DeleteExpiredVPC(ec2Session ec2.EC2, tagName string, dryRun bool) error {
	vpcs, err := listTaggedVPC(ec2Session, tagName)
	if err != nil {
		return fmt.Errorf("can't list VPC: %s\n", err)
	}

	_ = deleteVPC(ec2Session, vpcs, dryRun)

	return nil
}