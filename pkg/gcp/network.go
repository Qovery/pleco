package gcp

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"encoding/json"
	"fmt"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"strconv"
	"time"
)

func DeleteExpiredVPCs(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	networksIterator := sessions.Network.List(ctx, &computepb.ListNetworksRequest{
		Project: options.ProjectID,
	})

	for {
		network, err := networksIterator.Next()
		if err != nil {
			break
		}

		networkName := ""
		if network.Name != nil {
			networkName = *network.Name
		}
		networkDescription := ""
		if network.Description != nil {
			networkDescription = *network.Description
		}

		resourceTags := common.ResourceTags{}
		if err = json.Unmarshal([]byte(networkDescription), &resourceTags); err != nil {
			log.Info(fmt.Sprintf("No resource tags found in description field, ignoring this network (`%s`)", networkName))
			continue
		}
		ttlStr := ""
		if resourceTags.TTL != nil {
			ttlStr = *resourceTags.TTL
		} else {
			log.Info(fmt.Sprintf("No ttl value found, ignoring this network (`%s`)", networkName))
			continue
		}
		ttl, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this network (`%s`)", ttlStr, networkName))
			continue
		}
		creationTimeStr := ""
		if resourceTags.CreationUnixTimestamp != nil {
			creationTimeStr = *resourceTags.CreationUnixTimestamp
		} else {
			log.Info(fmt.Sprintf("No creation time value found, ignoring this network (`%s`)", networkName))
			continue
		}
		creationTimeInt64, err := strconv.ParseInt(creationTimeStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this network (`%s`)", creationTimeStr, networkName))
			continue
		}
		creationTime := time.Unix(creationTimeInt64, 0).UTC()

		// Network is not expired (or is protected TTL = 0)
		if ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Network `%s will be deleted`", networkName))
			continue
		}

		log.Info(fmt.Sprintf("Deleting network `%s`", networkName))
		_, err = sessions.Network.Delete(ctx, &computepb.DeleteNetworkRequest{
			Project: options.ProjectID,
			Network: networkName,
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error deleting network `%s`, error: %s", networkName, err))
		}
	}
}
