package aws

import (
	"time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
	log "github.com/sirupsen/logrus"
)

type stateMachine struct {
	ARN         string
	machineName string
	CreateTime  time.Time
	TTL         int64
	IsProtected bool
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
			ARN:         *machine.StateMachineArn,
			machineName: *machine.Name,
			CreateTime:  *machine.CreationDate,
			TTL:         essentialTags.TTL,
			IsProtected: essentialTags.IsProtected,
		})
	}

	return taggedMachines, nil
}

func deleteStateMachine(svc sfn.SFN, machine stateMachine) error {

	log.Infof("Deleting Step Function %s in %s, expired after %d seconds",
		machine.machineName, *svc.Config.Region, machine.TTL)

	_, err := svc.DeleteStateMachine(&sfn.DeleteStateMachineInput{
		StateMachineArn: &machine.ARN,
	},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredMachines(ECsession *sfn.SFN, tagName string) ([]stateMachine, string) {
	machines, err := listTaggedStateMachines(*ECsession, tagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("can't list Step Functions in region %s: %s", region, err.Error())
	}

	var expiredMachines []stateMachine
	for _, machine := range machines {
		if common.CheckIfExpired(machine.CreateTime, machine.TTL, "stateMachines: "+machine.machineName) && !machine.IsProtected {
			expiredMachines = append(expiredMachines, machine)
		}
	}

	return expiredMachines, region
}

func DeleteExpiredStateMachines(sessions AWSSessions, options AwsOptions) {
	expiredMachines, region := getExpiredMachines(sessions.SFN, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired Lambda Function", len(expiredMachines), region)

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
