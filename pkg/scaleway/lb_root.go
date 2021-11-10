package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type ScalewayLB struct {
	ID           string
	Name         string
	ClusterId string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredLBs(sessions *ScalewaySessions, options *ScalewayOptions) {
	expiredLBs, region := getExpiredLBs(sessions.Cluster, sessions.LoadBalancer, options.Region, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired load balancer", len(expiredLBs), region)

	log.Debug(count)

	if options.DryRun || len(expiredLBs) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredLB := range expiredLBs {
		deleteLB(sessions.LoadBalancer, options.Region, expiredLB)
	}
}

func getLbClusterId(tags []string) string {
	for _, tag := range tags {
		if strings.Contains(tag, "cluster="){
			val := strings.SplitN(tag, "=", 2)
			return val[1]
		}
	}

	return ""
}

func listUnlinkedLoadBalancers(lbs []ScalewayLB, clusters []ScalewayCluster) []ScalewayLB  {
	lbClusterIds := make(map[string]ScalewayLB)
	var unlinkedLbs []ScalewayLB
	for _, lb := range lbs {
		lbClusterIds[lb.ClusterId] = lb
	}

	for _, cluster := range clusters {
		lbClusterIds[cluster.ID] = ScalewayLB{Name: "linked"}
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
		loadBalancers = append(loadBalancers, ScalewayLB{
			ID:           lb.ID,
			Name:         lb.Name,
			ClusterId: getLbClusterId(lb.Tags),
			CreationDate: essentialTags.CreationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return loadBalancers, input.Region.String()
}

func getExpiredLBs(clusterAPI *k8s.API, lbAPI *lb.API, region scw.Region, tagName string) ([]ScalewayLB, string) {
	lbs , _ := listLBs(lbAPI, region, tagName)
	clusters, _ := ListClusters(clusterAPI, tagName)

	expiredLBs := []ScalewayLB{}
	for _, lb := range lbs {
		if common.CheckIfExpired(lb.CreationDate, lb.TTL, "load balancer"+lb.Name) && !lb.IsProtected {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	expiredLBs = append(expiredLBs, listUnlinkedLoadBalancers(lbs, clusters)...)

	return expiredLBs, region.String()
}

func deleteLB(lbAPI *lb.API, region scw.Region, loadBalancer ScalewayLB) {
	err := lbAPI.DeleteLB(
		&lb.DeleteLBRequest{
			LBID: loadBalancer.ID,
			Region: region,
			ReleaseIP: true,
		},
	)

	if err != nil {
		log.Errorf("Can't delete load balancer %s: %s", loadBalancer.Name, err.Error())
	}
}
