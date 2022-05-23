package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type stateMachine struct {
	common.CloudProviderResource
	machineName string
}

func stateMachineSession(sess session.Session, region string) *sfn.SFN {
	return sfn.New(&sess, &aws.Config{Region: aws.String(region)})
}

func listTaggedStateMachines(svc sfn.SFN, tagName string) ([]stateMachine, error) {
	var taggedMachines []stateMachine

	result, err := svc.ListStateMachines(nil)
	if err != nil {
		return nil, err
	}

	if len(result.StateMachines) == 0 {
		return nil, nil
	}

	for _, machine := range result.StateMachines {
		tags, err := svc.ListTagsForResource(
			&sfn.ListTagsForResourceInput{
				ResourceArn: aws.String(*machine.StateMachineArn),
			},
		)
		if err != nil {
			continue
		}

		essentialTags := common.GetEssentialTags(tags.Tags, tagName)

		taggedMachines = append(taggedMachines, stateMachine{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *machine.StateMachineArn,
				Description:  "State Machine: " + *machine.Name,
				CreationDate: *machine.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			machineName: *machine.Name,
		})
	}

	return taggedMachines, nil
}

func deleteStateMachine(svc sfn.SFN, machine stateMachine) error {

	log.Infof("Deleting Step Function %s in %s, expired after %d seconds",
		machine.machineName, *svc.Config.Region, machine.TTL)

	_, err := svc.DeleteStateMachine(&sfn.DeleteStateMachineInput{
		StateMachineArn: &machine.Identifier,
	},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredMachines(ECsession *sfn.SFN, options *AwsOptions) ([]stateMachine, string) {
	machines, err := listTaggedStateMachines(*ECsession, options.TagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("can't list Step Functions in region %s: %s", region, err.Error())
	}

	var expiredMachines []stateMachine
	for _, machine := range machines {
		if machine.IsResourceExpired(options.TagValue) {
			expiredMachines = append(expiredMachines, machine)
		}
	}

	return expiredMachines, region
}

func DeleteExpiredStateMachines(sessions AWSSessions, options AwsOptions) {
	expiredMachines, region := getExpiredMachines(sessions.SFN, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired Step Function", len(expiredMachines), region)

	log.Debug(count)

	if options.DryRun || len(expiredMachines) == 0 {
		return
	}

	log.Debug(start)

	for _, machine := range expiredMachines {
		deletionErr := deleteStateMachine(*sessions.SFN, machine)
		if deletionErr != nil {
			log.Errorf("Deletion Step function error %s/%s: %s", machine.machineName, region, deletionErr.Error())
		}
	}
}
