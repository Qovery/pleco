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
			} else {
				removeServiceAccountIAMBindings(sessions, options, serviceAccount.Email)
			}
		}

		nextPageToken = serviceAccountsListResponse.NextPageToken

		if serviceAccountsListResponse.NextPageToken == "" {
			break
		}
	}
}

const iamPolicyMemberLimit = 1500
const iamPolicyMemberChunkSize = 100

// removeServiceAccountIAMBindings removes all IAM policy bindings for the given
// service account email from the project policy. It is intended to be called
// immediately before or after deleting the service account so that dangling
// bindings are cleaned up atomically rather than relying on GCP eventually
// marking them as "deleted:serviceAccount:" prefixed entries.
func removeServiceAccountIAMBindings(sessions GCPSessions, options GCPOptions, serviceAccountEmail string) {
	member := "serviceAccount:" + serviceAccountEmail

	policy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Error(fmt.Sprintf("Error getting IAM policy for project `%s` while removing bindings for `%s`, error: %s", options.ProjectID, serviceAccountEmail, err))
		return
	}

	changed := false
	var updatedBindings []*cloudresourcemanager.Binding
	for _, binding := range policy.Bindings {
		var remainingMembers []string
		for _, m := range binding.Members {
			if m == member {
				log.Info(fmt.Sprintf("Removing IAM policy binding for deleted service account: role=%s member=%s", binding.Role, member))
				changed = true
				continue
			}
			remainingMembers = append(remainingMembers, m)
		}
		if len(remainingMembers) > 0 {
			binding.Members = remainingMembers
			updatedBindings = append(updatedBindings, binding)
		}
	}

	if !changed {
		return
	}

	policy.Bindings = updatedBindings
	if _, err = sessions.CRM.Projects.SetIamPolicy(options.ProjectID, &cloudresourcemanager.SetIamPolicyRequest{
		Policy: policy,
	}).Do(); err != nil {
		log.Error(fmt.Sprintf("Error updating IAM policy for project `%s` while removing bindings for `%s`, error: %s", options.ProjectID, serviceAccountEmail, err))
	}
}

func DeleteOrphanedIAMPolicyBindings(sessions GCPSessions, options GCPOptions) {
	policy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Error(fmt.Sprintf("Error getting IAM policy for project `%s`, error: %s", options.ProjectID, err))
		return
	}

	type orphanedBinding struct {
		role   string
		member string
	}

	var orphaned []orphanedBinding

	for _, binding := range policy.Bindings {
		for _, member := range binding.Members {
			if strings.HasPrefix(member, "deleted:serviceAccount:") {
				orphaned = append(orphaned, orphanedBinding{role: binding.Role, member: member})
				if options.DryRun {
					log.Info(fmt.Sprintf("Orphaned IAM policy binding will be removed: role=%s member=%s", binding.Role, member))
				} else {
					log.Info(fmt.Sprintf("Removing orphaned IAM policy binding: role=%s member=%s", binding.Role, member))
				}
			}
		}
	}

	if len(orphaned) == 0 {
		log.Info("No eligible orphaned IAM policy bindings found")
		return
	}

	if options.DryRun {
		log.Info("DryRun mode enabled, won't delete orphaned IAM policy bindings")
		return
	}

	for start := 0; start < len(orphaned); start += iamPolicyMemberChunkSize {
		end := start + iamPolicyMemberChunkSize
		if end > len(orphaned) {
			end = len(orphaned)
		}
		chunk := orphaned[start:end]

		currentPolicy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
		if err != nil {
			log.Error(fmt.Sprintf("Error getting IAM policy for project `%s`, error: %s", options.ProjectID, err))
			return
		}

		toRemove := make(map[string]map[string]bool)
		for _, ob := range chunk {
			if toRemove[ob.role] == nil {
				toRemove[ob.role] = make(map[string]bool)
			}
			toRemove[ob.role][ob.member] = true
		}

		var updatedBindings []*cloudresourcemanager.Binding
		for _, binding := range currentPolicy.Bindings {
			var remainingMembers []string
			for _, member := range binding.Members {
				if toRemove[binding.Role] != nil && toRemove[binding.Role][member] {
					continue
				}
				remainingMembers = append(remainingMembers, member)
			}
			if len(remainingMembers) > 0 {
				binding.Members = remainingMembers
				updatedBindings = append(updatedBindings, binding)
			}
		}

		totalMembers := 0
		for _, b := range updatedBindings {
			totalMembers += len(b.Members)
		}
		if totalMembers > iamPolicyMemberLimit {
			log.Warn(fmt.Sprintf(
				"Skipping SetIamPolicy for project `%s`: resulting policy would have %d members (limit %d). Reduce existing members first.",
				options.ProjectID, totalMembers, iamPolicyMemberLimit,
			))
			return
		}

		currentPolicy.Bindings = updatedBindings
		if _, err = sessions.CRM.Projects.SetIamPolicy(options.ProjectID, &cloudresourcemanager.SetIamPolicyRequest{
			Policy: currentPolicy,
		}).Do(); err != nil {
			log.Error(fmt.Sprintf("Error updating IAM policy for project `%s`, error: %s", options.ProjectID, err))
			return
		}

		log.Info(fmt.Sprintf("Removed %d orphaned IAM policy bindings for project `%s` (batch %d/%d)",
			len(chunk), options.ProjectID, end/iamPolicyMemberChunkSize, (len(orphaned)+iamPolicyMemberChunkSize-1)/iamPolicyMemberChunkSize))
	}
}
