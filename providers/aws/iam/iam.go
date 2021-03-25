package iam

import (
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
)

func DeleteExpiredIAM (iamSession *iam.IAM, tagName string, dryRun bool) {
	log.Info("Starting expired users scan.")
	DeleteExpiredUsers(iamSession, tagName, dryRun)

	log.Info("Starting expired roles scan.")
	DeleteExpiredRoles(iamSession, tagName, dryRun)

	log.Info("Starting detached policies scan.")
	DeleteDetachedPolicies(iamSession, dryRun)
}