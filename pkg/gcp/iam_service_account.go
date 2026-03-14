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
	"google.golang.org/api/googleapi"
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
const iamPolicyMaxRetries = 6

// updateIAMPolicyWithRetry performs a read-modify-write on the project IAM policy,
// retrying on 409 (concurrent modification) with exponential backoff.
// The mutate function receives the current policy and returns whether it was changed.
func updateIAMPolicyWithRetry(sessions GCPSessions, projectID string, mutate func(*cloudresourcemanager.Policy) bool) error {
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt <= iamPolicyMaxRetries; attempt++ {
		policy, err := sessions.CRM.Projects.GetIamPolicy(projectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
		if err != nil {
			return fmt.Errorf("getting IAM policy: %w", err)
		}

		if !mutate(policy) {
			return nil
		}

		_, err = sessions.CRM.Projects.SetIamPolicy(projectID, &cloudresourcemanager.SetIamPolicyRequest{
			Policy: policy,
		}).Do()
		if err == nil {
			return nil
		}

		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 409 {
			if attempt == iamPolicyMaxRetries {
				return err
			}
			log.Debug(fmt.Sprintf("IAM policy conflict for project `%s`, retrying after %s (attempt %d/%d)", projectID, backoff, attempt+1, iamPolicyMaxRetries))
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		return err
	}
	return nil
}

func removeServiceAccountIAMBindings(sessions GCPSessions, options GCPOptions, serviceAccountEmail string) {
	member := "serviceAccount:" + serviceAccountEmail

	err := updateIAMPolicyWithRetry(sessions, options.ProjectID, func(policy *cloudresourcemanager.Policy) bool {
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
		policy.Bindings = updatedBindings
		return changed
	})
	if err != nil {
		log.Error(fmt.Sprintf("Error updating IAM policy for project `%s` while removing bindings for `%s`, error: %s", options.ProjectID, serviceAccountEmail, err))
	}
}

func DeleteBindingsWithNonExistentServiceAccounts(sessions GCPSessions, options GCPOptions) {
	existingSAs := make(map[string]struct{})
	nextPageToken := ""
	for {
		resp, err := sessions.IAM.Projects.ServiceAccounts.List("projects/" + options.ProjectID).
			PageToken(nextPageToken).PageSize(100).Do()
		if err != nil {
			log.Error(fmt.Sprintf("Error listing service accounts for project `%s`, error: %s", options.ProjectID, err))
			return
		}
		for _, sa := range resp.Accounts {
			existingSAs["serviceAccount:"+sa.Email] = struct{}{}
		}
		nextPageToken = resp.NextPageToken
		if resp.NextPageToken == "" {
			break
		}
	}

	policy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Error(fmt.Sprintf("Error getting IAM policy for project `%s`, error: %s", options.ProjectID, err))
		return
	}

	nonExistent := make(map[string]struct{})
	for _, binding := range policy.Bindings {
		for _, member := range binding.Members {
			if strings.HasPrefix(member, "serviceAccount:") {
				if _, exists := existingSAs[member]; !exists {
					nonExistent[member] = struct{}{}
				}
			}
		}
	}

	if len(nonExistent) == 0 {
		log.Info("All serviceAccount members in IAM policy still exist, nothing to remove")
		return
	}

	if options.DryRun {
		for member := range nonExistent {
			log.Info(fmt.Sprintf("DryRun: would remove all IAM bindings for non-existent service account `%s`", member))
		}
		return
	}

	err = updateIAMPolicyWithRetry(sessions, options.ProjectID, func(policy *cloudresourcemanager.Policy) bool {
		changed := false
		var updatedBindings []*cloudresourcemanager.Binding
		for _, binding := range policy.Bindings {
			var remainingMembers []string
			for _, member := range binding.Members {
				if _, remove := nonExistent[member]; remove {
					log.Info(fmt.Sprintf("Removing IAM binding for non-existent service account: role=%s member=%s", binding.Role, member))
					changed = true
					continue
				}
				remainingMembers = append(remainingMembers, member)
			}
			if len(remainingMembers) > 0 {
				binding.Members = remainingMembers
				updatedBindings = append(updatedBindings, binding)
			}
		}
		policy.Bindings = updatedBindings
		return changed
	})
	if err != nil {
		log.Error(fmt.Sprintf("Error updating IAM policy for project `%s`, error: %s", options.ProjectID, err))
		return
	}

	log.Info(fmt.Sprintf("Removed IAM bindings for %d non-existent service accounts in project `%s`", len(nonExistent), options.ProjectID))
}

func DeleteOrphanedIAMPolicyBindings(sessions GCPSessions, options GCPOptions) {
	type orphanedBinding struct {
		role   string
		member string
	}

	var orphaned []orphanedBinding

	policy, err := sessions.CRM.Projects.GetIamPolicy(options.ProjectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Error(fmt.Sprintf("Error getting IAM policy for project `%s`, error: %s", options.ProjectID, err))
		return
	}

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
		batchNum := end / iamPolicyMemberChunkSize
		totalBatches := (len(orphaned) + iamPolicyMemberChunkSize - 1) / iamPolicyMemberChunkSize

		toRemove := make(map[string]map[string]bool)
		for _, ob := range chunk {
			if toRemove[ob.role] == nil {
				toRemove[ob.role] = make(map[string]bool)
			}
			toRemove[ob.role][ob.member] = true
		}

		err := updateIAMPolicyWithRetry(sessions, options.ProjectID, func(p *cloudresourcemanager.Policy) bool {
			var updatedBindings []*cloudresourcemanager.Binding
			for _, binding := range p.Bindings {
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
				return false
			}

			p.Bindings = updatedBindings
			return true
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error updating IAM policy for project `%s`, error: %s", options.ProjectID, err))
			return
		}

		log.Info(fmt.Sprintf("Removed %d orphaned IAM policy bindings for project `%s` (batch %d/%d)",
			len(chunk), options.ProjectID, batchNum, totalBatches))
	}
}
