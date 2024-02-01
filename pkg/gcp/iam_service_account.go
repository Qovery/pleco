package gcp

import (
	"encoding/json"
	"fmt"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

func DeleteExpiredServiceAccounts(sessions GCPSessions, options GCPOptions) {
	serviceAccountsListResponse, err := sessions.IAM.Projects.ServiceAccounts.List("projects/" + options.ProjectID).Do()
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
			ttlStr = *resourceTags.TTL
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
			creationTimeStr = *resourceTags.CreationUnixTimestamp
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
			log.Info(fmt.Sprintf("Service account `%s will be deleted`", serviceAccount.Name))
			continue
		}

		log.Info(fmt.Sprintf("Deleting service account `%s`", serviceAccount.Name))

		if _, err = sessions.IAM.Projects.ServiceAccounts.Delete(serviceAccount.Name).Do(); err != nil {
			log.Error(fmt.Sprintf("Error deleting service account `%s`, error: %s", serviceAccount.Name, err))
		}
	}
}
