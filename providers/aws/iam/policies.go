package iam

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"strconv"
)

func getPolicies(iamSession *iam.IAM) []*iam.Policy {
	result, err := iamSession.ListPolicies(
		&iam.ListPoliciesInput{
			MaxItems: aws.Int64(1000),
		})

	if err != nil {
		log.Error(err)
		return nil
	}

	return result.Policies
}

func DeleteDetachedPolicies(iamSession *iam.IAM, dryRun bool) error {
	policies := getPolicies(iamSession)
	var detachedPolicies []*iam.Policy

	for _, policy := range policies {
		if *policy.AttachmentCount == 0 {
			detachedPolicies = append(detachedPolicies, policy)
		}
	}

	log.Info("There is " + strconv.FormatInt(int64(len(detachedPolicies)),10) + " detached policies to delete.")

	if dryRun {
		return nil
	}

	for _, expiredPolicy := range detachedPolicies {
		_, err := iamSession.DeletePolicy(
			&iam.DeletePolicyInput{
				PolicyArn: aws.String(*expiredPolicy.Arn),
			})

		if err != nil {
			return err
		}
	}

	return nil
}
