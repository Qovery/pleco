package aws

import (
	log "github.com/sirupsen/logrus"
)

func DeleteExpiredIAM(sessions *AWSSessions, options *AwsOptions) {
	log.Info("Listing all IAM users.")
	DeleteExpiredUsers(sessions, options)

	log.Info("Listing all IAM roles.")
	DeleteExpiredRoles(sessions, options)

	log.Info("Listing all IAM policies.")
	DeleteDetachedPolicies(sessions, options.DryRun)

	log.Info("Listing all IAM instance profiles.")
	DeleteExpiredInstanceProfiles(sessions, options)
}
