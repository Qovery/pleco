package gcp

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
)

const maxNetworkDeletionWorkers = 20

// Network expires from the TTL metadata in its JSON description.
type Network struct {
	*computepb.Network
}

func (n Network) IsExpired() bool {
	return n.Network != nil && isExpiredFromDescription("network", n.GetName(), n.GetDescription())
}

func DeleteExpiredVPCs(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	networksIterator := sessions.Network.List(ctx, &computepb.ListNetworksRequest{
		Project: options.ProjectID,
	})

	// Finish listing before deletion waits can exhaust the listing timeout.
	var networks []Network
	for {
		networkI, err := networksIterator.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			log.Errorf("Error listing networks: %s", err)
			return
		}

		network := Network{networkI}
		if !network.IsExpired() {
			continue
		}
		networks = append(networks, network)
	}
	cancel()

	var wg sync.WaitGroup
	workers := make(chan struct{}, maxNetworkDeletionWorkers)
	for _, network := range networks {
		networkName := network.GetName()
		if options.DryRun {
			log.Infof("Network `%s` will be deleted", networkName)
			continue
		}

		// Acquire before spawning so at most 20 network goroutines exist at once.
		workers <- struct{}{}
		wg.Add(1)
		go func(network *computepb.Network) {
			defer wg.Done()
			defer func() { <-workers }()
			deleteVPC(sessions, options, network)
		}(network.Network)
	}
	wg.Wait()
}

func deleteVPC(sessions GCPSessions, options GCPOptions, network *computepb.Network) {
	networkName := network.GetName()
	// Cloud Routers (including their NAT configuration) pin the VPC and subnets.
	if err := deleteNetworkRouters(sessions, options, network); err != nil {
		log.Errorf("Skipping network `%s`: %s", networkName, err)
		return
	}

	networkFilter := fmt.Sprintf("network = \"%s\"", network.GetSelfLink())

	// Delete routes
	log.Info(fmt.Sprintf("Get routes for `%s`", network.GetName()))
	ctxListRoutes, cancelListRoutes := context.WithTimeout(context.Background(), 5*time.Minute)
	routesIterator := sessions.Route.List(ctxListRoutes, &computepb.ListRoutesRequest{
		Project: options.ProjectID,
		Filter:  &networkFilter,
	})
	for {
		route, err := routesIterator.Next()
		if err != nil {
			if err != iterator.Done {
				log.Errorf("Error listing routes for network `%s`: %s", networkName, err)
			}
			break
		}

		// Only custom static routes need explicit deletion.
		if !isCustomStaticRoute(route) {
			continue
		}

		if route.GetNetwork() == network.GetSelfLink() {
			log.Info(fmt.Sprintf("Deleting route `%s`", route.GetName()))
			ctxDeleteRoute, cancelDeleteRoute := context.WithTimeout(context.Background(), time.Second*60)
			operation, err := sessions.Route.Delete(ctxDeleteRoute, &computepb.DeleteRouteRequest{
				Project: options.ProjectID,
				Route:   route.GetName(),
			})
			if err != nil {
				log.Error(fmt.Sprintf("Error deleting route `%s`, error: %s", route.GetName(), err))
			}

			// this operation can be a bit long, we wait until it's done
			if operation != nil {
				err = operation.Wait(ctxDeleteRoute)
				if err != nil {
					log.Error(fmt.Sprintf("Error waiting for deleting route `%s`, error: %s", route.GetName(), err))
				}
			}

			// closing contexts
			cancelDeleteRoute()
		}
	}

	cancelListRoutes()

	// Delete firewall rules
	// GKE auto-creates firewall rules (e.g. `k8s-*-node-http-hc`, `gke-*`) that
	// outlive the cluster when cluster deletion fails mid-way, and they pin the VPC.
	// Listing has its own timeout because per-rule deletion waits can be long.
	log.Info(fmt.Sprintf("Get firewall rules for `%s`", network.GetName()))
	ctxListFirewalls, cancelListFirewalls := context.WithTimeout(context.Background(), time.Minute*5)
	firewallsIterator := sessions.Firewall.List(ctxListFirewalls, &computepb.ListFirewallsRequest{
		Project: options.ProjectID,
		Filter:  &networkFilter,
	})
	for {
		firewall, err := firewallsIterator.Next()
		if err != nil {
			if err != iterator.Done {
				log.Errorf("Error listing firewalls for network `%s`: %s", networkName, err)
			}
			break
		}

		if firewall.GetNetwork() == network.GetSelfLink() {
			log.Info(fmt.Sprintf("Deleting firewall rule `%s`", firewall.GetName()))
			ctxDeleteFirewall, cancelDeleteFirewall := context.WithTimeout(context.Background(), time.Second*60)
			operation, err := sessions.Firewall.Delete(ctxDeleteFirewall, &computepb.DeleteFirewallRequest{
				Project:  options.ProjectID,
				Firewall: firewall.GetName(),
			})
			if err != nil {
				log.Error(fmt.Sprintf("Error deleting firewall rule `%s`, error: %s", firewall.GetName(), err))
			}

			if operation != nil {
				err = operation.Wait(ctxDeleteFirewall)
				if err != nil {
					log.Error(fmt.Sprintf("Error waiting for deleting firewall rule `%s`, error: %s", firewall.GetName(), err))
				}
			}

			cancelDeleteFirewall()
		}
	}
	cancelListFirewalls()

	// GCP deletes auto-created subnets with an auto-mode network.
	// Custom-mode subnets must be removed explicitly.
	if !network.GetAutoCreateSubnetworks() {
		log.Info(fmt.Sprintf("Get subnets for `%s`", network.GetName()))
		ctxListSubnets, cancelListSubnets := context.WithTimeout(context.Background(), 5*time.Minute)
		subnetworksIterator := sessions.Subnetwork.AggregatedList(ctxListSubnets, &computepb.AggregatedListSubnetworksRequest{
			Project: options.ProjectID,
			Filter:  &networkFilter,
		})

		for {
			subnetworks, err := subnetworksIterator.Next()
			if err != nil {
				if err != iterator.Done {
					log.Errorf("Error listing subnets for network `%s`: %s", networkName, err)
				}
				break
			}
			if subnetworks.Value == nil || subnetworks.Value.Subnetworks == nil {
				continue
			}

			for _, subnetwork := range subnetworks.Value.Subnetworks {
				// Delete all subnets before deleting the network
				region, err := extractResourceRegion(subnetwork.GetRegion())
				if err != nil {
					log.Error(fmt.Sprintf("Error extracting region from subnet `%s`, error: %s", subnetwork.GetName(), err))
					continue
				}

				log.Info(fmt.Sprintf("Deleting subnet `%s` from region `%s`", subnetwork.GetName(), region))

				ctxDeleteSubnetwork, cancelDeleteSubnetwork := context.WithTimeout(context.Background(), time.Second*60)
				operation, err := sessions.Subnetwork.Delete(ctxDeleteSubnetwork, &computepb.DeleteSubnetworkRequest{
					Project:    options.ProjectID,
					Subnetwork: subnetwork.GetName(),
					Region:     region,
				})
				if err != nil {
					log.Error(fmt.Sprintf("Error deleting subnet `%s` from region `%s`, error: %s", subnetwork.GetName(), region, err))
				}

				// this operation can be a bit long, we wait until it's done
				if operation != nil {
					err = operation.Wait(ctxDeleteSubnetwork)
					if err != nil {
						log.Error(fmt.Sprintf("Error waiting for deleting subnet `%s` from region `%s`, error: %s", subnetwork.GetName(), region, err))
					}
				}

				// closing contexts
				cancelDeleteSubnetwork()
			}
		}
		cancelListSubnets()
	}

	log.Info(fmt.Sprintf("Deleting network `%s`", networkName))
	ctxDeleteNetwork, cancelNetwork := context.WithTimeout(context.Background(), time.Second*120)
	operation, err := sessions.Network.Delete(ctxDeleteNetwork, &computepb.DeleteNetworkRequest{
		Project: options.ProjectID,
		Network: networkName,
	})
	if err != nil {
		log.Error(fmt.Sprintf("Error deleting network `%s`, error: %s", networkName, err))
	}

	// this operation can be a bit long, we wait until it's done
	if operation != nil {
		err = operation.Wait(ctxDeleteNetwork)
		if err != nil {
			log.Error(fmt.Sprintf("Error waiting for deleting network `%s`, error: %s", network.GetName(), err))
		}
	}

	cancelNetwork()
}

func isCustomStaticRoute(route *computepb.Route) bool {
	// Imported and subnet routes are managed through their owning resources.
	if route.GetNextHopNetwork() != "" || route.GetNextHopPeering() != "" || route.GetNextHopHub() != "" {
		return false
	}
	switch route.GetRouteType() {
	case computepb.Route_STATIC.String():
	case "":
		// Older list responses may omit routeType. Require a static next hop.
		if route.GetNextHopGateway() == "" && route.GetNextHopInstance() == "" &&
			route.GetNextHopIp() == "" && route.GetNextHopIlb() == "" && route.GetNextHopVpnTunnel() == "" {
			return false
		}
	default:
		return false
	}

	// The API has no system-generated flag. Match the default internet route's
	// name and configuration; a custom default route via a VPN/VM still needs deletion.
	isDefaultInternetRoute := strings.HasPrefix(route.GetName(), "default-route-") &&
		(route.GetDestRange() == "0.0.0.0/0" || route.GetDestRange() == "::/0") &&
		strings.HasSuffix(route.GetNextHopGateway(), "/global/gateways/default-internet-gateway") &&
		route.GetPriority() == 1000 && len(route.GetTags()) == 0
	return !isDefaultInternetRoute
}
