package gcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
)

func DeleteExpiredServiceAccounts(sessions GCPSessions, options GCPOptions) {
	nextPageToken := ""
	for {
		serviceAccountsListResponse, err := sessions.IAM.Projects.ServiceAccounts.List("projects/" + options.ProjectID).
			PageToken(nextPageToken).PageSize(100).Do()
		if err != nil {
			log.Error(fmt.Sprintf("Error listing service accounts, error: %s", err))
			return
		}

		for _, serviceAccount := range serviceAccountsListResponse.Accounts {
			resourceTags := common.ResourceTags{}
			if err = json.Unmarshal([]byte(serviceAccount.Description), &resourceTags); err != nil {
				log.Info(fmt.Sprintf("No resource tags found in description field, ignoring this service account (`%s`)", serviceAccount.Name))
				continue
			}

			ttlStr := ""
			if resourceTags.TTL != nil {
				ttlStr = resourceTags.TTL.String()
			} else {
				log.Info(fmt.Sprintf("No ttl value found, ignoring this service account (`%s`)", serviceAccount.Name))
				continue
			}

			ttl, err := strconv.ParseInt(ttlStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this service account (`%s`)", ttlStr, serviceAccount.Name))
				continue
			}

			creationTimeStr := ""
			if resourceTags.CreationUnixTimestamp != nil {
				creationTimeStr = resourceTags.CreationUnixTimestamp.String()
			} else {
				log.Info(fmt.Sprintf("No creation time value found, ignoring this service account (`%s`)", serviceAccount.Name))
				continue
			}

			creationTimeInt64, err := strconv.ParseInt(creationTimeStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this service account (`%s`)", creationTimeStr, serviceAccount.Name))
				continue
			}

			creationTime := time.Unix(creationTimeInt64, 0).UTC()

			// Service account is not expired (or is protected TTL = 0)
			if ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
				continue
			}

			if options.DryRun {
				log.Info(fmt.Sprintf("Service account `%s` will be deleted", serviceAccount.Name))
				continue
			}

			log.Info(fmt.Sprintf("Deleting service account `%s`", serviceAccount.Name))

			if _, err = sessions.IAM.Projects.ServiceAccounts.Delete(serviceAccount.Name).Do(); err != nil {
				log.Error(fmt.Sprintf("Error deleting service account `%s`, error: %s", serviceAccount.Name, err))
			}
		}

		nextPageToken = serviceAccountsListResponse.NextPageToken

		// Break if there are no more pages
		if serviceAccountsListResponse.NextPageToken == "" {
			break
		}
	}
}

func DeleteOrphanedIAMPolicyBindings(sessions GCPSessions, options GCPOptions) {
	policy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Error(fmt.Sprintf("Error getting IAM policy for project `%s`, error: %s", options.ProjectID, err))
		return
	}

	var updatedBindings []*cloudresourcemanager.Binding
	hasChanges := false

	for _, binding := range policy.Bindings {
		var remainingMembers []string
		for _, member := range binding.Members {
			if strings.HasPrefix(member, "deleted:serviceAccount:") {
				hasChanges = true
				if options.DryRun {
					log.Info(fmt.Sprintf("Orphaned IAM policy binding will be removed: role=%s member=%s", binding.Role, member))
				} else {
					log.Info(fmt.Sprintf("Removing orphaned IAM policy binding: role=%s member=%s", binding.Role, member))
				}
				continue
			}
			remainingMembers = append(remainingMembers, member)
		}
		if len(remainingMembers) > 0 {
			binding.Members = remainingMembers
			updatedBindings = append(updatedBindings, binding)
		}
	}

	if !hasChanges {
		log.Info("No eligible orphaned IAM policy bindings found")
		return
	}

	if options.DryRun {
		log.Info("DryRun mode enabled, won't delete orphaned IAM policy bindings")
		return
	}

	policy.Bindings = updatedBindings
	if _, err = sessions.CRM.Projects.SetIamPolicy(options.ProjectID, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Do(); err != nil {
		log.Error(fmt.Sprintf("Error updating IAM policy for project `%s`, error: %s", options.ProjectID, err))
	}
}
