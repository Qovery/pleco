package do

import (
	"context"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"
)

type DOLB struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredLBs(sessions *DOSessions, options *DOOptions) {
	expiredLBs, region := getExpiredLBs(sessions.Client, options.TagName, options.Region)

	count, start := common.ElemToDeleteFormattedInfos("expired load balancer", len(expiredLBs), region)

	log.Debug(count)

	if options.DryRun || len(expiredLBs) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredLB := range expiredLBs {
		deleteLB(sessions.Client, expiredLB)
	}
}

func listLBs(client *godo.Client, tagName string, region string) []DOLB {
	result, _, err := client.LoadBalancers.List(context.TODO(), &godo.ListOptions{})

	if err != nil {
		log.Errorf("Can't list load balancers in region %s: %s", region, err.Error())
		return []DOLB{}
	}

	loadBalancers := []DOLB{}
	for _, lb := range result {
		essentialTags := common.GetEssentialTags(lb.Tags, tagName)
		loadBalancers = append(loadBalancers, DOLB{
			ID:           lb.ID,
			Name:         lb.Name,
			CreationDate: essentialTags.CreationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return loadBalancers
}

func getExpiredLBs(client *godo.Client, tagName string, region string) ([]DOLB, string) {
	lbs := listLBs(client, tagName, region)

	expiredLBs := []DOLB{}
	for _, lb := range lbs {
		if common.CheckIfExpired(lb.CreationDate, lb.TTL, "load balancer"+lb.Name) && !lb.IsProtected {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	return expiredLBs, region
}

func deleteLB(client *godo.Client, loadBalancer DOLB) {
	_, err := client.LoadBalancers.Delete(context.TODO(), loadBalancer.ID)

	if err != nil {
		log.Errorf("Can't delete load balancer %s: %s", loadBalancer.Name, err.Error())
	}
}
