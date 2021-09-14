package aws

import (
	"fmt"
	"github.com/Qovery/pleco/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/rds"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
	"time"
)

type eksCluster struct {
	ClusterCreateTime     time.Time
	ClusterName           string
	ClusterId             string
	ClusterNodeGroupsName []*string
	Status                string
	TTL                   int64
	IsProtected           bool
}

func AuthenticateToEks(clusterName string, clusterUrl string, roleArn string, session *session.Session) (*kubernetes.Clientset, error) {
	clusterApi := &api.Cluster{Server: clusterUrl}
	clusters := make(map[string]*api.Cluster)
	clusters[clusterName] = clusterApi
	c := &api.Config{Clusters: clusters}

	g, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create iam-authenticator token generator: %v", err)
	}

	t, err := g.GetWithRoleForSession("eks_test", roleArn, session)
	if err != nil {
		return nil, fmt.Errorf("failed to get token for eks: %v", err)
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*c, &clientcmd.ConfigOverrides{Context: api.Context{Cluster: clusterName}, AuthInfo: api.AuthInfo{Token: t.Token}})
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client set: %v", err)
	}
	return clientSet, nil
}

func listTaggedEKSClusters(svc eks.EKS, tagName string) ([]eksCluster, error) {
	var taggedClusters []eksCluster
	region := *svc.Config.Region

	input := &eks.ListClustersInput{}
	result, err := svc.ListClusters(input)
	if err != nil {
		return nil, err
	}

	if len(result.Clusters) == 0 {
		return nil, nil
	}

	for _, cluster := range result.Clusters {
		currentCluster := eks.DescribeClusterInput{
			Name: aws.String(*cluster),
		}
		clusterName := *currentCluster.Name

		clusterInfo, err := svc.DescribeCluster(&currentCluster)
		if err != nil {
			log.Errorf("Error while trying to get info from cluster %v (%s)", clusterName, region)
			continue
		}

		creationDate, ttl, isProtected, _, _ := utils.GetEssentialTags(clusterInfo.Cluster.Tags, tagName)

		// ignore if creation is in progress to avoid nil fields
		if *clusterInfo.Cluster.Status == "CREATING" {
			log.Debugf("Can't perform action on cluster %v (%s), current status is: %s", clusterName, region, *clusterInfo.Cluster.Status)
			continue
		}

		// get node groups
		nodeGroups, err := svc.ListNodegroups(&eks.ListNodegroupsInput{
			ClusterName: &clusterName,
		})
		if err != nil {
			log.Errorf("Error while trying to get node groups from cluster %s (%s): %s", clusterName, region, err)
			continue
		}

		taggedClusters = append(taggedClusters, eksCluster{
			ClusterCreateTime:     creationDate,
			ClusterNodeGroupsName: nodeGroups.Nodegroups,
			ClusterName:           clusterName,
			ClusterId:             utils.AwsStringChecker(clusterInfo.Cluster.Identity),
			Status:                *clusterInfo.Cluster.Status,
			TTL:                   ttl,
			IsProtected:           isProtected,
		})
	}

	return taggedClusters, nil
}

func deleteEKSCluster(svc eks.EKS, ec2Session ec2.EC2, elbSession elbv2.ELBV2, cloudwatchLogsSession cloudwatchlogs.CloudWatchLogs, rdsSession rds.RDS, cluster eksCluster, tagName string, dryRun bool) error {
	if cluster.Status == "DELETING" {
		log.Infof("EKS cluster %s (%s) is already in deletion process, skipping...", cluster.ClusterName, *svc.Config.Region)
		return nil
	} else if cluster.Status == "CREATING" {
		log.Infof("EKS cluster %s (%s) is in creating process, skipping...", cluster.ClusterName, *svc.Config.Region)
		return nil
	} else {
		log.Infof("Deleting EKS cluster %s (%s), expired after %d seconds",
			cluster.ClusterName, *svc.Config.Region, cluster.TTL)
	}

	if dryRun {
		return nil
	}

	// delete node groups
	if len(cluster.ClusterNodeGroupsName) > 0 {
		for _, nodeGroupName := range cluster.ClusterNodeGroupsName {
			nodeGroupStatus, _ := getNodeGroupStatus(svc, cluster, *nodeGroupName)

			if nodeGroupStatus == "DELETING" {
				log.Infof("EKS cluster nodegroup %v (%s) is already in deletion process, skipping...", *nodeGroupName, cluster.ClusterName)
				continue
			} else if nodeGroupStatus == "CREATING" {
				log.Infof("EKS cluster nodegroup %v (%s) is in creating process, skipping...", *nodeGroupName, cluster.ClusterName)
				continue
			} else {
				log.Infof("Deleting EKS cluster nodegroup %v (%s)", *nodeGroupName, cluster.ClusterName)
			}

			err := deleteNodeGroupStatus(svc, cluster, *nodeGroupName, dryRun)
			if err != nil {
				return fmt.Errorf("Error while deleting node group %v: %s\n", *nodeGroupName, err)
			}
		}
	}

	// as requests are asynchronous, we'll wait next run to perform delete and avoid obvious failure
	// because of nodes groups are not yet deleted
	if len(cluster.ClusterNodeGroupsName) > 0 {
		return nil
	}

	// tag associated ebs for deletion
	expiredELB, err := ListExpiredLoadBalancers(svc, elbSession, tagName)
	if err != nil {
		return err
	}
	err = TagLoadBalancersForDeletion(elbSession, tagName, expiredELB, cluster.ClusterName)
	if err != nil {
		return err
	}

	// tag associated ebs for deletion
	err = TagVolumesFromEksClusterForDeletion(ec2Session, tagName, cluster.ClusterName)
	if err != nil {
		return err
	}

	// tag cloudwatch logs for deletion
	err = TagLogsForDeletion(cloudwatchLogsSession, tagName, cluster.ClusterId, cluster.TTL)
	if err != nil {
		return err
	}

	// delete EKS cluster
	_, err = svc.DeleteCluster(
		&eks.DeleteClusterInput{
			Name: &cluster.ClusterName,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getNodeGroupStatus(svc eks.EKS, cluster eksCluster, nodeGroupName string) (string, error) {
	result, err := svc.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   &cluster.ClusterName,
		NodegroupName: &nodeGroupName,
	})
	if err != nil {
		return "", err
	}

	return *result.Nodegroup.Status, nil
}

func deleteNodeGroupStatus(svc eks.EKS, cluster eksCluster, nodeGroupName string, dryRun bool) error {
	if dryRun {
		return nil
	}

	_, err := svc.DeleteNodegroup(&eks.DeleteNodegroupInput{
		ClusterName:   &cluster.ClusterName,
		NodegroupName: &nodeGroupName,
	})
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpiredEKSClusters(svc eks.EKS, ec2Session ec2.EC2, elbSession elbv2.ELBV2, cloudwatchLogsSession cloudwatchlogs.CloudWatchLogs, rdsSession rds.RDS, tagName string, dryRun bool) {
	clusters, err := listTaggedEKSClusters(svc, tagName)
	region := svc.Config.Region
	if err != nil {
		log.Errorf("can't list EKS clusters: %s\n", err)
		return
	}

	var expiredCluster []eksCluster
	for _, cluster := range clusters {
		if utils.CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL, "eks cluster: "+cluster.ClusterName) && !cluster.IsProtected {
			expiredCluster = append(expiredCluster, cluster)
		}
	}

	count, start := utils.ElemToDeleteFormattedInfos("expired EKS cluster", len(expiredCluster), *region)

	log.Debug(count)

	if dryRun || len(expiredCluster) == 0 {
		return
	}

	log.Debug(start)

	for _, cluster := range expiredCluster {
		deletionErr := deleteEKSCluster(svc, ec2Session, elbSession, cloudwatchLogsSession, rdsSession, cluster, tagName, dryRun)
		if deletionErr != nil {
			log.Errorf("Deletion EKS cluster error %s/%s: %s",
				cluster.ClusterName, *region, deletionErr)
		}

	}
}
