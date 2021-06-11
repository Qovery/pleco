package aws

import (
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

func DeleteExpiredIAM (iamSession *iam.IAM, tagName string, dryRun bool) {
	log.Debug("Listing all IAM users.")
	DeleteExpiredUsers(iamSession, tagName, dryRun)

	log.Debug("Listing all IAM roles.")
	DeleteExpiredRoles(iamSession, tagName, dryRun)

	log.Debug("Listing all IAM policies.")
	DeleteDetachedPolicies(iamSession, dryRun)
}