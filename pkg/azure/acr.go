package azure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// DeleteExpiredACRs identifies and deletes Azure Container Registries that have expired based on their TTL tags
func DeleteExpiredACRs(sessions AzureSessions, options AzureOptions) {
	// Create a context with timeout to prevent hanging operations
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Ensure we have a valid ACR client
	if sessions.ACR == nil {
		log.Error("Container Registry client is not initialized")
		return
	}

	// List all container registries using pagination
	pager := sessions.ACR.NewListPager(nil)
	for pager.More() {
		// Fetch the next page of container registries
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Errorf("Error listing container registries, error: %v", err)
			return
		}

		// Process each container registry on the current page
		for _, registry := range page.Value {
			// Check if registry has a TTL tag
			ttlStr, ok := registry.Tags["ttl"]
			if !ok || strings.TrimSpace(*ttlStr) == "" {
				log.Info(fmt.Sprintf("No ttl label value found, ignoring this registry (`%s`)", *registry.Name))
				continue
			}

			// Parse the TTL value to int64
			ttl, err := strconv.ParseInt(*ttlStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this registry (`%s`)", *ttlStr, *registry.Name))
				continue
			}

			// Check if registry has a creation_date tag
			creationTimeStr, ok := registry.Tags["creation_date"]
			if !ok || strings.TrimSpace(*creationTimeStr) == "" {
				log.Info(fmt.Sprintf("No creation_date label value found, ignoring this registry (`%s`)", *registry.Name))
				continue
			}

			// Parse the creation_date value to int64 (Unix timestamp)
			creationTimeInt64, err := strconv.ParseInt(*creationTimeStr, 10, 64)
			if err != nil {
				log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this registry (`%s`)", *ttlStr, *registry.Name))
				continue
			}

			// Convert Unix timestamp to UTC time
			creationTime := time.Unix(creationTimeInt64, 0).UTC()
			
			// Skip if the registry is not expired or has TTL=0 (protected)
			// TTL=0 means the registry should never be automatically deleted
			if !ok || ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
				continue
			}

			// In dry run mode, just log what would be deleted without actually deleting
			if options.DryRun {
				log.Info(fmt.Sprintf("Container Registry `%s` will be deleted", *registry.Name))
				continue
			}

			// Parse resource group from ID
			// ID format: /subscriptions/{subId}/resourceGroups/{resourceGroup}/providers/Microsoft.ContainerRegistry/registries/{name}
			idParts := strings.Split(*registry.ID, "/")
			resourceGroupIndex := -1
			for i, part := range idParts {
				if strings.EqualFold(part, "resourceGroups") && i+1 < len(idParts) {
					resourceGroupIndex = i + 1
					break
				}
			}
			
			if resourceGroupIndex == -1 || resourceGroupIndex >= len(idParts) {
				log.Errorf("Could not extract resource group from registry ID: %s", *registry.ID)
				continue
			}
			
			resourceGroupName := idParts[resourceGroupIndex]

			// Delete the expired container registry
			log.Info(fmt.Sprintf("Deleting container registry `%s` created at `%s` UTC (TTL `{%d}` seconds)", *registry.Name, creationTime.UTC(), ttl))
			if _, err := sessions.ACR.BeginDelete(ctx, resourceGroupName, *registry.Name, nil); err != nil {
				log.Error(fmt.Sprintf("Error deleting container registry `%s`, error: %s", *registry.Name, err))
			}
		}
	}
}