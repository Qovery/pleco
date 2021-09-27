package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayLB struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredLBs(sessions *ScalewaySessions, options *ScalewayOption) {
	expiredLBs, region := getExpiredLBs(sessions.LoadBalancer, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired load balancer", len(expiredLBs), region)

	log.Debug(count)

	if options.DryRun || len(expiredLBs) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredLB := range expiredLBs {
		deleteLB(sessions.LoadBalancer, expiredLB)
	}
}

func listLBs(lbAPI *lb.API, tagName string) ([]ScalewayLB, string) {
	input := &lb.ListLBsRequest{}
	result, err := lbAPI.ListLBs(input)
	if err != nil {
		log.Errorf("Can't list load balancers in region %s: %s", input.Region.String(), err.Error())
	}

	loadBalancers := []ScalewayLB{}
	for _, lb := range result.LBs {
		creationDate, ttl, isProtected, _, _ := common.GetEssentialTags(lb.Tags, tagName)
		loadBalancers = append(loadBalancers, ScalewayLB{
			ID:           lb.ID,
			Name:         lb.Name,
			CreationDate: creationDate,
			TTL:          ttl,
			IsProtected:  isProtected,
		})
	}

	return loadBalancers, input.Region.String()
}

func getExpiredLBs(lbAPI *lb.API, tagName string) ([]ScalewayLB, string) {
	lbs, region := listLBs(lbAPI, tagName)

	expiredLBs := []ScalewayLB{}
	for _, lb := range lbs {
		if common.CheckIfExpired(lb.CreationDate, lb.TTL, "load balancer"+lb.Name) && !lb.IsProtected {
			expiredLBs = append(expiredLBs, lb)
		}
	}

	return expiredLBs, region
}

func deleteLB(lbAPI *lb.API, loadBalancer ScalewayLB) {
	err := lbAPI.DeleteLB(
		&lb.DeleteLBRequest{
			LBID: loadBalancer.ID,
		},
	)

	if err != nil {
		log.Errorf("Can't delete load balancer %s", loadBalancer.Name)
	}
}
