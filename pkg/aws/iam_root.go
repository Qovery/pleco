package aws

import (
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

func DeleteExpiredIAM(iamSession *iam.IAM, options *AwsOptions) {
	log.Info("Listing all IAM users.")
	DeleteExpiredUsers(iamSession, options)

	log.Info("Listing all IAM roles.")
	DeleteExpiredRoles(iamSession, options)

	log.Info("Listing all IAM policies.")
	DeleteDetachedPolicies(iamSession, options.DryRun)
}
