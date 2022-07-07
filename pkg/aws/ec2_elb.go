package aws

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elbv2"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type ElasticLoadBalancer struct {
	common.CloudProviderResource
	Arn    string
	Status string
	VpcId  string
	Tags   []*elbv2.Tag
}

func TagLoadBalancersForDeletion(lbSession *elbv2.ELBV2, tagKey string, loadBalancersList []ElasticLoadBalancer, clusterName string) error {
	var lbArns []*string

	if len(loadBalancersList) == 0 {
		return nil
	}

	for _, lb := range loadBalancersList {
		lbArns = append(lbArns, aws.String(lb.Arn))
	}

	if len(lbArns) == 0 {
		return nil
	}

	for _, lbArn := range lbArns {
		_, err := lbSession.AddTags(
			&elbv2.AddTagsInput{
				ResourceArns: aws.StringSlice([]string{*lbArn}),
				Tags: []*elbv2.Tag{
					{
						Key:   aws.String(tagKey),
						Value: aws.String("1"),
					},
					{
						Key:   aws.String("creationDate"),
						Value: aws.String(time.Now().Format(time.RFC3339)),
					},
				},
			},
		)
		if err != nil {
			return fmt.Errorf("Can't tag load balancer %s for cluster %s in region %s: %s", *lbArn, clusterName, *lbSession.Config.Region, err.Error())
		}
	}

	return nil
}

func ListExpiredLoadBalancers(eksSession *eks.EKS, lbSession *elbv2.ELBV2, options *AwsOptions) ([]ElasticLoadBalancer, error) {
	var taggedLoadBalancers []ElasticLoadBalancer

	allLoadBalancers, err := ListLoadBalancers(lbSession, options.TagName)
	if err != nil {
		return nil, fmt.Errorf("Error while getting loadbalancer list on region %s\n", *lbSession.Config.Region)
	}

	if len(allLoadBalancers) == 0 {
		return nil, nil
	}

	for _, currentLb := range allLoadBalancers {
		if !currentLb.IsProtected && (!common.IsAssociatedToLivingCluster(currentLb.Tags, eksSession) || currentLb.IsResourceExpired(options.TagValue, options.DisableTTLCheck)) {
			taggedLoadBalancers = append(taggedLoadBalancers, currentLb)
		}
	}

	return taggedLoadBalancers, nil
}

func ListLoadBalancers(lbSession *elbv2.ELBV2, tagName string) ([]ElasticLoadBalancer, error) {
	var allLoadBalancers []ElasticLoadBalancer

	input := elbv2.DescribeLoadBalancersInput{}

	result, err := lbSession.DescribeLoadBalancers(&input)
	if err != nil {
		return nil, err
	}

	if len(result.LoadBalancers) == 0 {
		return nil, nil
	}

	region := *lbSession.Config.Region
	for _, currentLb := range result.LoadBalancers {
		input := elbv2.DescribeTagsInput{ResourceArns: []*string{currentLb.LoadBalancerArn}}
		result, err := lbSession.DescribeTags(&input)
		currentLbName := *currentLb.LoadBalancerName

		if err != nil {
			log.Errorf("Error while getting load balancer tags from %s in %s", currentLbName, region)
			continue
		}
		loadBalancerTags := result.TagDescriptions[0].Tags
		essentialTags := common.GetEssentialTags(loadBalancerTags, tagName)

		allLoadBalancers = append(allLoadBalancers, ElasticLoadBalancer{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   currentLbName,
				Description:  "EC2 ELB: " + currentLbName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Arn:    *currentLb.LoadBalancerArn,
			Status: *currentLb.State.Code,
			VpcId:  *currentLb.VpcId,
			Tags:   loadBalancerTags,
		})
	}

	return allLoadBalancers, nil
}

func deleteLoadBalancers(lbSession *elbv2.ELBV2, loadBalancersList []ElasticLoadBalancer, dryRun bool) {
	if dryRun {
		return
	}

	if len(loadBalancersList) == 0 {
		return
	}

	for _, lb := range loadBalancersList {
		log.Infof("Deleting ELB %s in %s", lb.Identifier, *lbSession.Config.Region)
		_, err := lbSession.DeleteLoadBalancer(
			&elbv2.DeleteLoadBalancerInput{LoadBalancerArn: &lb.Arn},
		)
		if err != nil {
			log.Errorf("Can't delete ELB %s in %s", lb.Identifier, *lbSession.Config.Region)
		}
	}
}

func DeleteExpiredLoadBalancers(sessions AWSSessions, options AwsOptions) {
	expiredLoadBalancers, err := ListExpiredLoadBalancers(sessions.EKS, sessions.ELB, &options)
	region := *sessions.ELB.Config.Region
	if err != nil {
		log.Errorf("can't list Load Balancers: %s\n", err)
		return
	}

	count, start := common.ElemToDeleteFormattedInfos("expired ELB load balancer", len(expiredLoadBalancers), region)

	log.Debug(count)

	if options.DryRun || len(expiredLoadBalancers) == 0 {
		return
	}

	log.Debug(start)

	deleteLoadBalancers(sessions.ELB, expiredLoadBalancers, options.DryRun)
}

func getLoadBalancerByVpId(lbSession *elbv2.ELBV2, vpc VpcInfo) ElasticLoadBalancer {
	lbs, err := ListLoadBalancers(lbSession, "")
	if err != nil {
		log.Errorf("can't list Load Balancers: %s\n", err)
		return ElasticLoadBalancer{}
	}

	for _, lb := range lbs {
		if lb.VpcId == vpc.Identifier {
			return lb
		}
	}

	return ElasticLoadBalancer{}
}

func DeleteLoadBalancerByVpcId(lbSession *elbv2.ELBV2, vpc VpcInfo, dryRun bool) {
	lb := getLoadBalancerByVpId(lbSession, vpc)
	if lb.Arn != "" {
		deleteLoadBalancers(lbSession, []ElasticLoadBalancer{lb}, dryRun)
		time.Sleep(30 * time.Second)
	}
}
