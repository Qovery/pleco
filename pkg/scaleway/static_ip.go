package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	log "github.com/sirupsen/logrus"
)

type ScalewayIP struct {
	ID      string
	Address string
}

func DeleteOrphanIPAddresses(sessions ScalewaySessions, options ScalewayOptions) {
	orphanIPs, _ := getOrphanIPAddresses(sessions.LoadBalancer, options)

	count, start := common.ElemToDeleteFormattedInfos("orphan IP address", len(orphanIPs), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(orphanIPs) == 0 {
		return
	}

	log.Info(start)

	for _, orphanIP := range orphanIPs {
		log.Info("Deleting orphan IP address: ", orphanIP.Address)
		if err := sessions.LoadBalancer.ReleaseIP(&lb.ZonedAPIReleaseIPRequest{
			Zone: scw.Zone(options.Zone),
			IPID: orphanIP.ID,
		}); err != nil {
			log.Errorf("Error deleting IP %s: %v", orphanIP.Address, err)
		}
	}
}

func getOrphanIPAddresses(lbAPI *lb.ZonedAPI, options ScalewayOptions) ([]ScalewayIP, error) {
	// List all IPs
	orphanIPs := make([]ScalewayIP, 0)
	var page int32 = 1
	var itemsPerPage uint32 = 100

	for {
		ipsResponse, err := lbAPI.ListIPs(&lb.ZonedAPIListIPsRequest{
			Page:     &page,
			PageSize: &itemsPerPage,
			Zone:     scw.Zone(options.Zone),
		})
		if err != nil {
			log.Fatalf("Error listing IPs: %v", err)
			return nil, err
		}

		for _, ip := range ipsResponse.IPs {
			if ip.LBID == nil { // Check if IP is not attached to any LB
				orphanIPs = append(orphanIPs, ScalewayIP{ID: ip.ID, Address: ip.IPAddress})
			}
		}

		if ipsResponse.TotalCount <= uint32(page)*itemsPerPage {
			break
		}

		page += 1
	}

	return orphanIPs, nil
}
