package iam

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

type Policy struct {
	Name string
	Arn string
}

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

func DeleteDetachedPolicies(iamSession *iam.IAM, dryRun bool) {
	policies := getPolicies(iamSession)
	var detachedPolicies []*iam.Policy

	for _, policy := range policies {
		if *policy.AttachmentCount == 0 {
			detachedPolicies = append(detachedPolicies, policy)
		}
	}

	s := "There is no detached IAM policy to delete."
	if len(detachedPolicies) == 1 {
		s = "There is 1 detached IAM policy to delete."
	}
	if len(detachedPolicies) > 1 {
		s = fmt.Sprintf("There are %d detached IAM policies to delete.", len(detachedPolicies))
	}

	log.Debug(s)

	if dryRun || len(detachedPolicies) == 0 {
		return
	}

	log.Debug("Starting detached policies deletion.")

	for _, expiredPolicy := range detachedPolicies {
		_, err := iamSession.DeletePolicy(
			&iam.DeletePolicyInput{
				PolicyArn: aws.String(*expiredPolicy.Arn),
			})

		if err != nil {
			log.Errorf("Can't delete policy %s : %s", *expiredPolicy.PolicyName, err.Error())
		}
	}
}

func getUserPolicies(iamSession *iam.IAM, userName string) []Policy {
	attachedPolicies, policyErr := iamSession.ListAttachedUserPolicies(
		&iam.ListAttachedUserPoliciesInput{
			MaxItems: aws.Int64(1000),
			UserName: aws.String(userName),
		})

	policyNames, namesErr := iamSession.ListUserPolicies(
		&iam.ListUserPoliciesInput{
			MaxItems: aws.Int64(1000),
			UserName: aws.String(userName),
		})

	if policyErr != nil {
		log.Error(policyErr)
		return nil
	}

	if namesErr != nil {
		log.Error(namesErr)
		return nil
	}

	var userPolicies []Policy
	for _, policy := range attachedPolicies.AttachedPolicies{
		userPolicy := Policy{
			Arn: *policy.PolicyArn,
			Name: *policy.PolicyName,
		}
		userPolicies = append(userPolicies, userPolicy)
	}

	for _, policyName := range policyNames.PolicyNames{
		userPolicy := Policy{
			Arn: "",
			Name: *policyName,
		}
		userPolicies = append(userPolicies, userPolicy)
	}


	return userPolicies
}

func detachUserPolicies(iamSession *iam.IAM, userName string, policies []Policy) {
	for _, policy := range policies {
		if policy.Arn != "" {
			_, err :=iamSession.DetachUserPolicy(
				&iam.DetachUserPolicyInput{
					UserName: aws.String(userName),
					PolicyArn: aws.String(policy.Arn),
				})

			if err != nil {
				continue
			}
		}


	}
}

func deleteUserPolicies(iamSession *iam.IAM, userName string, policies []Policy) {
	for _, policy := range policies {
		_, err := iamSession.DeleteUserPolicy(
			&iam.DeleteUserPolicyInput{
				UserName: aws.String(userName),
				PolicyName: aws.String(policy.Name),
			})

		if err != nil {
			continue
		}
	}
}

func HandleUserPolicies(iamSession *iam.IAM, userName string) {
	policies := getUserPolicies(iamSession, userName)
	deleteUserPolicies(iamSession, userName, policies)
	detachUserPolicies(iamSession, userName, policies)
}

func getRolePolicies(iamSession *iam.IAM, roleName string) []Policy {
	attachedPolicies, policyErr := iamSession.ListAttachedRolePolicies(
		&iam.ListAttachedRolePoliciesInput{
			MaxItems: aws.Int64(1000),
			RoleName: aws.String(roleName),
		})

	policyNames, namesErr := iamSession.ListRolePolicies(
		&iam.ListRolePoliciesInput{
			MaxItems: aws.Int64(1000),
			RoleName: aws.String(roleName),
		})

	if policyErr != nil {
		log.Error(policyErr)
		return nil
	}

	if namesErr != nil {
		log.Error(namesErr)
		return nil
	}

	var rolePolicies []Policy
	for _, policy := range attachedPolicies.AttachedPolicies{
		userPolicy := Policy{
			Arn: *policy.PolicyArn,
			Name: *policy.PolicyName,
		}
		rolePolicies = append(rolePolicies, userPolicy)
	}

	for _, policyName := range policyNames.PolicyNames{
		userPolicy := Policy{
			Arn: "",
			Name: *policyName,
		}
		rolePolicies = append(rolePolicies, userPolicy)
	}

	return rolePolicies
}


func detachRolePolicies(iamSession *iam.IAM, roleName string, policies []Policy) {
	for _, policy := range policies {
		if policy.Arn != "" {
			_, err := iamSession.DetachRolePolicy(
				 &iam.DetachRolePolicyInput{
					 RoleName: aws.String(roleName),
					 PolicyArn: aws.String(policy.Arn),
				 })
			if err != nil {
				continue
			}
		 }
	}
}

func deleteRolePolicies(iamSession *iam.IAM, roleName string, policies []Policy) {
	for _, policy := range policies {
		_, err := iamSession.DeleteRolePolicy(
			&iam.DeleteRolePolicyInput{
				RoleName: aws.String(roleName),
				PolicyName: aws.String(policy.Name),
			})
		if err != nil {
			continue
		}
	}
}

func HandleRolePolicies(iamSession *iam.IAM, roleName string) {
	policies := getRolePolicies(iamSession, roleName)
	deleteRolePolicies(iamSession, roleName, policies)
	detachRolePolicies(iamSession, roleName, policies)
}
