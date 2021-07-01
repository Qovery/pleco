package aws

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"
	"strings"
)

type ElasticLoadBalancer struct {
	Arn string
	Name string
	Status string
	IsProtected bool
}

func listExpiredLoadBalancers(eksSession eks.EKS,lbSession elbv2.ELBV2, tagName string) ([]ElasticLoadBalancer, error) {
	var taggedLoadBalancers []ElasticLoadBalancer
	region := *lbSession.Config.Region

	allLoadBalancers, err := ListLoadBalancers(lbSession)
	if err != nil {
		return nil, fmt.Errorf("Error while getting loadbalancer list on region %s\n", *lbSession.Config.Region)
	}

	if len(allLoadBalancers) == 0 {
		return nil, nil
	}

	for _, currentLb := range allLoadBalancers {
		input := elbv2.DescribeTagsInput{ResourceArns: []*string{&currentLb.Arn}}

		result, err := lbSession.DescribeTags(&input)
		if err != nil {
			log.Errorf("Error while getting load balancer tags from %s in %s", currentLb.Name, region)
			continue
		}

		_, _, isProtected, _, _ := utils.GetEssentialTags(result.TagDescriptions[0].Tags, tagName)
		currentLb.IsProtected = isProtected

		if !isAssociatedToLivingCluster(result.TagDescriptions[0].Tags, eksSession) && !currentLb.IsProtected {
			taggedLoadBalancers = append(taggedLoadBalancers, currentLb)
		}
	}

	return taggedLoadBalancers, nil
}

func isAssociatedToLivingCluster(tags []*elbv2.Tag, svc eks.EKS) bool {
	result, clusterErr := svc.ListClusters(&eks.ListClustersInput{})
	if clusterErr != nil {
		log.Error("Can't list cluster for ELB association check")
		return false
	}

	for _, cluster := range result.Clusters {
		for _, tag := range tags {
			if strings.Contains(*tag.Key, "/cluster/") && strings.Contains(*tag.Key, *cluster) {
				return true
			}
		}
	}

	return false
}

func ListLoadBalancers(lbSession elbv2.ELBV2) ([]ElasticLoadBalancer, error) {
	var allLoadBalancers []ElasticLoadBalancer

	input := elbv2.DescribeLoadBalancersInput{}

	result, err := lbSession.DescribeLoadBalancers(&input)
	if err != nil {
		return nil, err
	}

	if len(result.LoadBalancers) == 0 {
		return nil, nil
	}

	for _, currentLb := range result.LoadBalancers {
		allLoadBalancers = append(allLoadBalancers, ElasticLoadBalancer{
			Arn:         *currentLb.LoadBalancerArn,
			Name:        *currentLb.LoadBalancerName,
			Status:      *currentLb.State.Code,
		})
	}

	return allLoadBalancers, nil
}

func deleteLoadBalancers(lbSession elbv2.ELBV2, loadBalancersList []ElasticLoadBalancer, dryRun bool) {
	if dryRun {
		return
	}

	if len(loadBalancersList) == 0 {
		return
	}

	for _, lb := range loadBalancersList {
		log.Infof("Deleting ELB %s in %s", lb.Name, *lbSession.Config.Region)
		_, err := lbSession.DeleteLoadBalancer(
			&elbv2.DeleteLoadBalancerInput{LoadBalancerArn: &lb.Arn},
		)
		if err != nil {
			log.Errorf("Can't delete ELB %s in %s", lb.Name, *lbSession.Config.Region)
		}
	}
}

func DeleteExpiredLoadBalancers(eksSession eks.EKS, elbSession elbv2.ELBV2, tagName string, dryRun bool) {
	expiredLoadBalancers, err := listExpiredLoadBalancers(eksSession, elbSession, tagName)
	region := elbSession.Config.Region
	if err != nil {
		log.Errorf("can't list Load Balancers: %s\n", err)
		return
	}

	count, start:= utils.ElemToDeleteFormattedInfos("expired ELB load balancer", len(expiredLoadBalancers), *region)

	log.Debug(count)

	if dryRun || len(expiredLoadBalancers) == 0 {
		return
	}

	log.Debug(start)

	deleteLoadBalancers(elbSession, expiredLoadBalancers, dryRun)
}