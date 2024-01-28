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
		vpc, err := networksIterator.Next()
		if err != nil {
			break
		}

		vpcName := ""
		if vpc.Name != nil {
			vpcName = *vpc.Name
		}
		vpcDescription := ""
		if vpc.Description != nil {
			vpcDescription = *vpc.Description
		}

		resourceTags := common.ResourceTags{}
		if err = json.Unmarshal([]byte(vpcDescription), &resourceTags); err != nil {
			log.Info(fmt.Sprintf("No resource tags found in description field, ignoring this network (`%s`)", vpcName))
			continue
		}
		ttl, err := strconv.ParseInt(resourceTags.TTL, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this network (`%s`)", resourceTags.TTL, vpcName))
			continue
		}
		creationTimeInt64, err := strconv.ParseInt(resourceTags.CreationUnixTimestamp, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this network (`%s`)", resourceTags.TTL, vpcName))
			continue
		}
		creationTime := time.Unix(creationTimeInt64, 0).UTC()

		// Network is not expired (or is protected TTL = 0)
		if ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Network `%s will be deleted`", vpcName))
			continue
		}

		log.Info(fmt.Sprintf("Deleting network `%s`", vpcName))
		_, err = sessions.Network.Delete(ctx, &computepb.DeleteNetworkRequest{
			Project: options.ProjectID,
			Network: vpcName,
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error deleting network `%s`, error: %s", vpcName, err))
		}
	}
}
