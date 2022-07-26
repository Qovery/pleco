package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
	"os"

	"github.com/Qovery/pleco/pkg/common"
)

type EC2Instance struct {
	common.CloudProviderResource
}

func deleteEC2Instances(ec2Session *ec2.EC2, ec2Instances []EC2Instance) {
	for _, ec2Instance := range ec2Instances {
		instanceIds := []*string{&ec2Instance.Identifier}
		_, err := ec2Session.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: instanceIds,
		})
		if err != nil {
			log.Errorf("Can't delete %s in %s", ec2Instance.Identifier, *ec2Session.Config.Region)
		}
	}
}

func listExpiredEC2Instances(ec2Session *ec2.EC2, options *AwsOptions) ([]EC2Instance, error) {
	result, err := ec2Session.DescribeInstances(&ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}

	if len(result.Reservations) == 0 {
		return nil, nil
	}

	var expiredEC2Instances []EC2Instance
	for _, currentReservation := range result.Reservations {
		for _, ec2Instance := range currentReservation.Instances {
			// available instance states listed here: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceState.html
			if *ec2Instance.State.Name != "running" {
				log.Infof("Skipping EC2 instance %s in region %s (current status is %s)", *ec2Instance.InstanceId, *ec2Session.Config.Region, *ec2Instance.State.Name)
				continue
			}

			if options.DisableTTLCheck {
				vpcId, isOk := os.LookupEnv("PROTECTED_VPC_ID")
				if !isOk {
					log.Fatalf("Unable to get PROTECTED_VPC_ID environment variable in order to protect VPC resources.")
				}
				if vpcId == *ec2Instance.VpcId {
					log.Infof("Skipping EC2 instance %s in region %s (protected vpc)", *ec2Instance.InstanceId, *ec2Session.Config.Region)
					continue
				}

			}

			essentialTags := common.GetEssentialTags(ec2Instance.Tags, options.TagName)
			ec2Instance := EC2Instance{
				CloudProviderResource: common.CloudProviderResource{
					Identifier:   *ec2Instance.InstanceId,
					Description:  "EC2 Instance: " + *ec2Instance.InstanceId,
					CreationDate: essentialTags.CreationDate,
					TTL:          essentialTags.TTL,
					Tag:          essentialTags.Tag,
					IsProtected:  essentialTags.IsProtected,
				},
			}

			if ec2Instance.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
				expiredEC2Instances = append(expiredEC2Instances, ec2Instance)
			}
		}
	}

	return expiredEC2Instances, nil
}

func DeleteExpiredEC2Instances(sessions AWSSessions, options AwsOptions) {
	expiredEC2Instances, err := listExpiredEC2Instances(sessions.EC2, &options)
	region := *sessions.EC2.Config.Region
	if err != nil {
		log.Errorf("Can't list instances: %s\n", err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("expired EC2 instance", len(expiredEC2Instances), region)

	log.Debug(count)

	if options.DryRun || len(expiredEC2Instances) == 0 {
		return
	}

	log.Debug(start)

	deleteEC2Instances(sessions.EC2, expiredEC2Instances)
}
