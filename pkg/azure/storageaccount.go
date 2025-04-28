package azure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// DeleteExpiredStorageAccounts identifies and deletes Azure Storage Accounts that have expired based on their TTL tags
func DeleteExpiredStorageAccounts(sessions AzureSessions, options AzureOptions) {
	// Create a context with timeout to prevent hanging operations
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Ensure we have a valid Storage Accounts client
	if sessions.StorageAccount == nil {
		log.Error("Storage Accounts client is not initialized")
		return
	}

	// List all storage accounts
	pager := sessions.StorageAccount.NewListPager(nil)
	for pager.More() {
		// Fetch the next page of storage accounts
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Errorf("Error listing storage accounts, error: %v", err)
			return
		}

		// Process each storage account on the current page
		for _, account := range page.Value {
			// Skip if no ID
			if account.ID == nil {
				continue
			}

			// Check if storage account has a TTL tag
			ttlStr, ok := account.Tags["ttl"]
			if !ok || ttlStr == nil || strings.TrimSpace(*ttlStr) == "" {
				log.Info(fmt.Sprintf("No ttl label value found, ignoring this storage account (`%s`)", *account.Name))
				continue
			}

			// Parse the TTL value to int64
			ttl, err := strconv.ParseInt(*ttlStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this storage account (`%s`)", *ttlStr, *account.Name))
				continue
			}

			// Check if storage account has a creation_date tag
			creationTimeStr, ok := account.Tags["creation_date"]
			if !ok || creationTimeStr == nil || strings.TrimSpace(*creationTimeStr) == "" {
				log.Info(fmt.Sprintf("No creation_date label value found, ignoring this storage account (`%s`)", *account.Name))
				continue
			}

			// Parse the creation_date value to int64 (Unix timestamp)
			creationTimeInt64, err := strconv.ParseInt(*creationTimeStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this storage account (`%s`)", *creationTimeStr, *account.Name))
				continue
			}

			// Convert Unix timestamp to UTC time
			creationTime := time.Unix(creationTimeInt64, 0).UTC()
			
			// Skip if the storage account is not expired or has TTL=0 (protected)
			// TTL=0 means the storage account should never be automatically deleted
			if ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
				continue
			}

			// Extract resource group name from ID
			// ID format: /subscriptions/{subId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Storage/storageAccounts/{accountName}
			idParts := strings.Split(*account.ID, "/")
			resourceGroupIndex := -1
			for i, part := range idParts {
				if strings.EqualFold(part, "resourceGroups") && i+1 < len(idParts) {
					resourceGroupIndex = i + 1
					break
				}
			}
			
			if resourceGroupIndex == -1 {
				log.Errorf("Could not extract resource group from storage account ID: %s", *account.ID)
				continue
			}
			
			resourceGroupName := idParts[resourceGroupIndex]

			// In dry run mode, just log what would be deleted without actually deleting
			if options.DryRun {
				log.Info(fmt.Sprintf("Storage Account `%s` in resource group `%s` will be deleted", *account.Name, resourceGroupName))
				continue
			}

			// Delete the expired storage account
			log.Info(fmt.Sprintf("Deleting storage account `%s` in resource group `%s` created at `%s` UTC (TTL `%d` seconds)", 
				*account.Name, resourceGroupName, creationTime.String(), ttl))
				
			_, err = sessions.StorageAccount.Delete(ctx, resourceGroupName, *account.Name, nil)
			if err != nil {
				log.Error(fmt.Sprintf("Error deleting storage account `%s`, error: %s", *account.Name, err))
			}
		}
	}
}
