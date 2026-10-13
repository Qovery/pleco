package gcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

// Router expires from the TTL metadata in its JSON description.
type Router struct {
	*computepb.Router
}

func (r Router) IsExpired() bool {
	return r.Router != nil && isExpiredFromDescription("router", r.GetName(), r.GetDescription())
}

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
		routerName := router.GetName()
		if !(Router{Router: router}).IsExpired() {
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
