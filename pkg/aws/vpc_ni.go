package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/sirupsen/logrus"
	"sync"
)

type NetworkInterface struct {
	Id           string
	VpcId        string
	AttachmentId string
}

func DeleteNetworkInterfacesByVpcId(ec2Session ec2.EC2, vpcId string) {
	NIs := listNetworkInterfacesByVpcId(ec2Session, vpcId)

	for _, ni := range NIs {

		if ni.AttachmentId != "" {
			detachNetworkInterfaces(ec2Session, ni)
		}
		deleteNetworkInterface(ec2Session, ni)
	}
}

func listNetworkInterfacesByVpcId(ec2Session ec2.EC2, vpcId string) []NetworkInterface {
	result, err := ec2Session.DescribeNetworkInterfaces(&ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
		},
	})
	if err != nil {
		logrus.Errorf("Can't list Network interface in region %s: %s.", *ec2Session.Config.Region, err.Error())
	}

	NIs := []NetworkInterface{}
	for _, ni := range result.NetworkInterfaces {
		NI := NetworkInterface{
			Id:    *ni.NetworkInterfaceId,
			VpcId: *ni.VpcId,
		}

		if ni.Attachment != nil {
			NI.AttachmentId = *ni.Attachment.AttachmentId
		}

		NIs = append(NIs, NI)
	}

	return NIs
}

func detachNetworkInterfaces(ec2Session ec2.EC2, ni NetworkInterface) {
	_, err := ec2Session.DetachNetworkInterface(
		&ec2.DetachNetworkInterfaceInput{
			AttachmentId: &ni.AttachmentId,
		})
	if err != nil {
		logrus.Errorf("Can't detach network interface %s: %s", ni.Id, err.Error())
	}
}

func deleteNetworkInterface(ec2Session ec2.EC2, ni NetworkInterface) {
	_, err := ec2Session.DeleteNetworkInterface(
		&ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(ni.Id),
		})
	if err != nil {
		logrus.Errorf("Can't delete network interface %s: %s", ni.Id, err.Error())
	}
}

func SetNetworkInterfacesByVpcId(ec2Session ec2.EC2, vpc *VpcInfo, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	vpc.NetworkInterfaces = listNetworkInterfacesByVpcId(ec2Session, *vpc.VpcId)
}
