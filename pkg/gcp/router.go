package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

func DeleteExpiredRouters(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	routersIterator := sessions.Router.List(ctx, &computepb.ListRoutersRequest{
		Project: options.ProjectID,
		Region:  options.Location,
	})

	// Finish listing before waiting for deletions, which can exceed the list timeout.
	var routers []*computepb.Router
	for {
		router, err := routersIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorf("Error listing routers in region `%s`: %s", options.Location, err)
			return
		}
		routers = append(routers, router)
	}
	cancel()

	for _, router := range routers {
		routerName := ""
		if router.Name != nil {
			routerName = *router.Name
		}
		routerDescription := ""
		if router.Description != nil {
			routerDescription = *router.Description
		}

		resourceTags := common.ResourceTags{}
		if err := json.Unmarshal([]byte(routerDescription), &resourceTags); err != nil {
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

		if err := deleteRouter(sessions, options.ProjectID, options.Location, routerName); err != nil {
			log.Error(err)
		}
	}
}

// Routers attached to a network selected for deletion are dependencies, just like
// its subnets and firewall rules. They do not require separate TTL metadata.
func deleteNetworkRouters(sessions GCPSessions, options GCPOptions, network *computepb.Network) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filter := fmt.Sprintf("network = %q", network.GetSelfLink())
	routersIterator := sessions.Router.AggregatedList(ctx, &computepb.AggregatedListRoutersRequest{
		Project: options.ProjectID,
		Filter:  &filter,
	})
	var routers []*computepb.Router
	for {
		scopedRouters, err := routersIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("listing routers for network `%s`: %w", network.GetName(), err)
		}
		if scopedRouters.Value == nil {
			continue
		}
		for _, router := range scopedRouters.Value.GetRouters() {
			if router.GetNetwork() == network.GetSelfLink() {
				routers = append(routers, router)
			}
		}
	}
	cancel()
	for _, router := range routers {
		region, err := extractResourceRegion(router.GetRegion())
		if err != nil {
			return fmt.Errorf("extracting region of router `%s`: %w", router.GetName(), err)
		}
		if err := deleteRouter(sessions, options.ProjectID, region, router.GetName()); err != nil {
			return err
		}
	}
	return nil
}

func deleteRouter(sessions GCPSessions, projectID, region, name string) error {
	log.Infof("Deleting router `%s` from region `%s`", name, region)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	operation, err := sessions.Router.Delete(ctx, &computepb.DeleteRouterRequest{
		Project: projectID,
		Region:  region,
		Router:  name,
	})
	if err != nil {
		// A regional router watcher may already have removed this dependency.
		var apiErr *googleapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("deleting router `%s` from region `%s`: %w", name, region, err)
	}
	if err := operation.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for deletion of router `%s` from region `%s`: %w", name, region, err)
	}
	return nil
}
