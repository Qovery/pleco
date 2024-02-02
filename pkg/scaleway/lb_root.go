package scaleway

import (
	"strings"

	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewayLB struct {
	common.CloudProviderResource
	Name      string
	ClusterId string
	PublicIps []string
}

func DeleteExpiredLBs(sessions ScalewaySessions, options ScalewayOptions) {
	expiredLBs, region := getExpiredLBs(sessions.Cluster, sessions.LoadBalancer, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired load balancer", len(expiredLBs), region)

	log.Info(count)

	if options.DryRun || len(expiredLBs) == 0 {
		return
	}

	log.Info(start)

	for _, expiredLB := range expiredLBs {
		deleteLB(sessions.LoadBalancer, scw.Zone(options.Zone), expiredLB)
	}
}

func getLbClusterId(tags []string) string {
	for _, tag := range tags {
		if strings.Contains(tag, "cluster=") {
			val := strings.SplitN(tag, "=", 2)
			return val[1]
		}
	}

	return ""
}

func listUnlinkedLoadBalancers(lbs []ScalewayLB, clusters []ScalewayCluster) []ScalewayLB {
	lbClusterIds := make(map[string]ScalewayLB)
	var unlinkedLbs []ScalewayLB
	for _, lb := range lbs {
		lbClusterIds[lb.ClusterId] = lb
	}

	for _, cluster := range clusters {
		lbClusterIds[cluster.Identifier] = ScalewayLB{Name: "linked"}
	}

	for _, lb := range lbClusterIds {
		if lb.Name != "linked" {
			unlinkedLbs = append(unlinkedLbs, lb)
		}
	}

	return unlinkedLbs
}

func listLBs(lbAPI *lb.ZonedAPI, zone scw.Zone, tagName string) ([]ScalewayLB, string) {
	input := &lb.ZonedAPIListLBsRequest{
		Zone: zone,
	}
	result, err := lbAPI.ListLBs(input)
	if err != nil {
		log.Errorf("Can't list load balancers in zone %s: %s", input.Zone.String(), err.Error())
		return []ScalewayLB{}, input.Zone.String()
	}

	loadBalancers := []ScalewayLB{}
	for _, lb := range result.LBs {
		essentialTags := common.GetEssentialTags(lb.Tags, tagName)
		var ips []string
		for _, ip := range lb.IP {
			ips = append(ips, ip.IPAddress)
		}
		loadBalancers = append(loadBalancers, ScalewayLB{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   lb.ID,
				Description:  "Load Balancer: " + lb.Name,
				CreationDate: lb.CreatedAt.UTC(),
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Name:      lb.Name,
			ClusterId: getLbClusterId(lb.Tags),
			PublicIps: ips,
		})
	}

	return loadBalancers, input.Zone.String()
}

func getExpiredLBs(clusterAPI *k8s.API, lbAPI *lb.ZonedAPI, options *ScalewayOptions) ([]ScalewayLB, string) {
	lbs, _ := listLBs(lbAPI, scw.Zone(options.Zone), options.TagName)
	clusters, _, err := ListClusters(clusterAPI, options.TagName)

	// Early return to avoid side effect in listUnlinkedLoadBalancers methods: as no clusters would be fetched,
	// the load balancers listed will be considered as unlinked therefore pleco will clean them
	if err != nil {
		log.Info("As list-clusters failed to be fetched, consider that no load-balancer is expired")
		return []ScalewayLB{}, options.Region.String()
	}

	expiredLBs := []ScalewayLB{}
	for _, lb := range lbs {
		if lb.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	expiredLBs = append(expiredLBs, listUnlinkedLoadBalancers(lbs, clusters)...)

	return expiredLBs, options.Region.String()
}

func deleteLB(lbAPI *lb.ZonedAPI, zone scw.Zone, loadBalancer ScalewayLB) {
	err := lbAPI.DeleteLB(
		&lb.ZonedAPIDeleteLBRequest{
			LBID:      loadBalancer.Identifier,
			Zone:      zone,
			ReleaseIP: true,
		},
	)

	if err != nil {
		log.Errorf("Can't delete load balancer %s: %s", loadBalancer.Name, err.Error())
	} else {
		log.Debugf("Load balancer %s: %v in %s deleted.", loadBalancer.Name, loadBalancer.PublicIps, zone)
	}
}
