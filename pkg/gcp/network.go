package gcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
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
			ttlStr = resourceTags.TTL.String()
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
			creationTimeStr = resourceTags.CreationUnixTimestamp.String()
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

		log.Info(fmt.Sprintf("Getting network `%s`", networkName))
		ctxGetNetwork, cancelGetNetwork := context.WithTimeout(context.Background(), time.Second*30)
		vpcToDelete, err := sessions.Network.Get(ctxGetNetwork, &computepb.GetNetworkRequest{
			Project: options.ProjectID,
			Network: networkName,
		})
		if err != nil {
			log.Error(fmt.Sprintf("Error getting network `%s`, error: %s", networkName, err))
		}

		if vpcToDelete != nil && vpcToDelete.AutoCreateSubnetworks != nil && *vpcToDelete.AutoCreateSubnetworks {
			log.Info(fmt.Sprintf("Converting network `%s` to subnet custom mode in order to be able to delete it", networkName))
			// Patch the network to custom mode allowing to delete subnets
			ctxSwitchToCustomMode, cancelSwitchToCustomMode := context.WithTimeout(context.Background(), time.Second*60)
			operation, err := sessions.Network.SwitchToCustomMode(ctxSwitchToCustomMode, &computepb.SwitchToCustomModeNetworkRequest{
				Project: options.ProjectID,
				Network: networkName,
			})
			if err != nil {
				log.Error(fmt.Errorf("failed to convert network %s to custom mode: %w", networkName, err))
				continue
			}

			// this operation can be a bit long, we wait until it's done
			err = operation.Wait(ctxSwitchToCustomMode)
			if err != nil {
				log.Error(fmt.Sprintf("Error waiting for convert network %s to custom mode: %s", networkName, err))
			}

			cancelSwitchToCustomMode()
		}

		log.Info(fmt.Sprintf("Get subnets for `%s`", network.GetName()))
		networkFilter := fmt.Sprintf("network = \"%s\"", network.GetSelfLink())
		subnetworksIterator := sessions.Subnetwork.AggregatedList(ctx, &computepb.AggregatedListSubnetworksRequest{
			Project: options.ProjectID,
			Filter:  &networkFilter,
		})

		for {
			subnetworks, err := subnetworksIterator.Next()
			if err != nil {
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

		// Delete routes
		log.Info(fmt.Sprintf("Get routes for `%s`", network.GetName()))
		routesIterator := sessions.Route.List(ctx, &computepb.ListRoutesRequest{
			Project: options.ProjectID,
			Filter:  &networkFilter,
		})
		for {
			route, err := routesIterator.Next()
			if err != nil {
				break
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

		// closing contexts
		cancelGetNetwork()

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
}
