package do

import (
	"context"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type DOLB struct {
	common.CloudProviderResource
	Name     string
	Droplets []int
}

func DeleteExpiredLBs(sessions DOSessions, options DOOptions) {
	expiredLBs := getExpiredLBs(sessions.Client, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired load balancer", len(expiredLBs), "")

	log.Debug(count)

	if options.DryRun || len(expiredLBs) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredLB := range expiredLBs {
		deleteLB(sessions.Client, expiredLB)
	}
}

func listLBs(client *godo.Client, tagName string) []DOLB {
	result, _, err := client.LoadBalancers.List(context.TODO(), &godo.ListOptions{PerPage: 200})

	if err != nil {
		log.Errorf("Can't list load balancers: %s", err.Error())
		return []DOLB{}
	}

	loadBalancers := []DOLB{}
	for _, lb := range result {
		creationDate, _ := time.Parse(time.RFC3339, lb.Created)
		essentialTags := common.GetEssentialTags(lb.Tags, tagName)
		loadBalancers = append(loadBalancers, DOLB{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   lb.ID,
				Description:  "Load Balancer: " + lb.Name,
				CreationDate: creationDate.UTC(),
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Name:     lb.Name,
			Droplets: lb.DropletIDs,
		})
	}

	return loadBalancers
}

func getExpiredLBs(client *godo.Client, options *DOOptions) []DOLB {
	lbs := listLBs(client, options.TagName)

	expiredLBs := []DOLB{}
	for _, lb := range lbs {
		if lb.IsResourceExpired(options.TagValue) {
			expiredLBs = append(expiredLBs, lb)
		}

		if len(lb.Droplets) == 0 {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	return expiredLBs
}

func deleteLB(client *godo.Client, loadBalancer DOLB) {
	_, err := client.LoadBalancers.Delete(context.TODO(), loadBalancer.Identifier)

	if err != nil {
		log.Errorf("Can't delete load balancer %s: %s", loadBalancer.Name, err.Error())
	}
}
