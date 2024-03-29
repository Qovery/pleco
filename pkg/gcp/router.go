package gcp

import (
	"cloud.google.com/go/compute/apiv1/computepb"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	"strconv"
	"time"
)

func DeleteExpiredRouters(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	routersIterator := sessions.Router.List(ctx, &computepb.ListRoutersRequest{
		Project: options.ProjectID,
		Region:  options.Location,
	})

	for {
		router, err := routersIterator.Next()
		if err != nil {
			break
		}

		routerName := ""
		if router.Name != nil {
			routerName = *router.Name
		}
		routerDescription := ""
		if router.Description != nil {
			routerDescription = *router.Description
		}

		resourceTags := common.ResourceTags{}
		if err = json.Unmarshal([]byte(routerDescription), &resourceTags); err != nil {
			log.Info(fmt.Sprintf("No resource tags found in description field, ignoring this router (`%s`)", routerName))
			continue
		}
		ttlStr := ""
		if resourceTags.TTL != nil {
			ttlStr = resourceTags.TTL.String()
		} else {
			log.Info(fmt.Sprintf("No ttl value found, ignoring this router (`%s`)", routerName))
			continue
		}
		ttl, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this router (`%s`)", ttlStr, routerName))
			continue
		}
		creationTimeStr := ""
		if resourceTags.CreationUnixTimestamp != nil {
			creationTimeStr = resourceTags.CreationUnixTimestamp.String()
		} else {
			log.Info(fmt.Sprintf("No creation time value found, ignoring this router (`%s`)", routerName))
			continue
		}
		creationTimeInt64, err := strconv.ParseInt(creationTimeStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this router (`%s`)", creationTimeStr, routerName))
			continue
		}
		creationTime := time.Unix(creationTimeInt64, 0).UTC()

		// Router is not expired (or is protected TTL = 0)
		if ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Router `%s will be deleted`", routerName))
			continue
		}

		log.Info(fmt.Sprintf("Deleting router `%s`", routerName))
		_, err = sessions.Router.Delete(ctx, &computepb.DeleteRouterRequest{
			Project: options.ProjectID,
			Region:  options.Location,
			Router:  routerName,
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error deleting router `%s`, error: %s", routerName, err))
		}
	}
}
