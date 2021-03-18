package iam

import (
	"github.com/aws/aws-sdk-go/service/iam"
)

func DeleteExpiredIAM (iamSession *iam.IAM, tagName string, dryRun bool) error {
	err := DeleteExpiredUsers(iamSession, tagName, dryRun)
	if err != nil {
		return err
	}

	err = DeleteExpiredRoles(iamSession, tagName, dryRun)
	if err != nil {
		return err
	}

	err = DeleteDetachedPolicies(iamSession, dryRun)
	if err != nil {
		return err
	}

	return nil
}