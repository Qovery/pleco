package azure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// DeleteExpiredRGs identifies and deletes Azure Resource Groups that have expired based on their TTL tags
func DeleteExpiredRGs(sessions AzureSessions, options AzureOptions) {
	// Create a context with timeout to prevent hanging operations
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Ensure we have a valid Resource Groups client
	if sessions.RG == nil {
		log.Error("Resource Groups client is not initialized")
		return
	}

	// List all resource groups using pagination
	pager := sessions.RG.NewListPager(nil)
	for pager.More() {
		// Fetch the next page of resource groups
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Errorf("Error listing clusters, error: %v", err)
			return
		}

		// Process each resource group on the current page
		for _, group := range page.Value {
			// Check if resource group has a TTL tag
			ttlStr, ok := group.Tags["ttl"]
			if !ok || strings.TrimSpace(*ttlStr) == "" {
				log.Info(fmt.Sprintf("No ttl label value found, ignoring this cluster (`%s`)", *group.Name))
				continue
			}

			// Parse the TTL value to int64
			ttl, err := strconv.ParseInt(*ttlStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this cluster (`%s`)", *ttlStr, *group.Name))
				continue
			}

			// Check if resource group has a creation_date tag
			creationTimeStr, ok := group.Tags["creation_date"]
			if !ok || strings.TrimSpace(*creationTimeStr) == "" {
				log.Info(fmt.Sprintf("No creation_date label value found, ignoring this cluster (`%s`)", *group.Name))
				continue
			}

			// Parse the creation_date value to int64 (Unix timestamp)
			creationTimeInt64, err := strconv.ParseInt(*creationTimeStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this cluster (`%s`)", *ttlStr, *group.Name))
				continue
			}

			// Convert Unix timestamp to UTC time
			creationTime := time.Unix(creationTimeInt64, 0).UTC()
			
			// Skip if the resource group is not expired or has TTL=0 (protected)
			// TTL=0 means the resource group should never be automatically deleted
			if !ok || ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
				continue
			}

			// In dry run mode, just log what would be deleted without actually deleting
			if options.DryRun {
				log.Info(fmt.Sprintf("Resource Group `%s` will be deleted", *group.Name))
				continue
			}

			// Delete the expired resource group
			log.Info(fmt.Sprintf("Deleting resource group `%s` created at `%s` UTC (TTL `{%d}` seconds)", *group.Name, creationTime.UTC(), ttl))
			if _, err := sessions.RG.BeginDelete(ctx, *group.Name, nil); err != nil {
				log.Error(fmt.Sprintf("Error deleting resource group `%s`, error: %s", *group.Name, err))
			}
		}
	}
}
