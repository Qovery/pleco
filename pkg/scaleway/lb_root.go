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
		deleteLB(sessions.LoadBalancer, options.Region, expiredLB)
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

func listLBs(lbAPI *lb.API, region scw.Region, tagName string) ([]ScalewayLB, string) {
	input := &lb.ListLBsRequest{
		Region: region,
	}
	result, err := lbAPI.ListLBs(input)
	if err != nil {
		log.Errorf("Can't list load balancers in region %s: %s", input.Region.String(), err.Error())
		return []ScalewayLB{}, input.Region.String()
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

	return loadBalancers, input.Region.String()
}

func getExpiredLBs(clusterAPI *k8s.API, lbAPI *lb.API, options *ScalewayOptions) ([]ScalewayLB, string) {
	lbs, _ := listLBs(lbAPI, options.Region, options.TagName)
	clusters, _ := ListClusters(clusterAPI, options.TagName)

	expiredLBs := []ScalewayLB{}
	for _, lb := range lbs {
		if lb.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	expiredLBs = append(expiredLBs, listUnlinkedLoadBalancers(lbs, clusters)...)

	return expiredLBs, options.Region.String()
}

func deleteLB(lbAPI *lb.API, region scw.Region, loadBalancer ScalewayLB) {
	err := lbAPI.DeleteLB(
		&lb.DeleteLBRequest{
			LBID:      loadBalancer.Identifier,
			Region:    region,
			ReleaseIP: true,
		},
	)

	if err != nil {
		log.Errorf("Can't delete load balancer %s: %s", loadBalancer.Name, err.Error())
	} else {
		log.Debugf("Load balancer %s: %v in %s deleted.", loadBalancer.Name, loadBalancer.PublicIps, region)
	}
}
