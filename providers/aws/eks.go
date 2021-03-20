package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
	"strconv"
	"time"
)

type eksCluster struct {
	ClusterCreateTime     time.Time
	ClusterName           string
	ClusterId             string
	ClusterNodeGroupsName []*string
	Status                string
	TTL                   int64
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

func listTaggedEKSClusters(svc *eks.EKS, tagName string) ([]eksCluster, error) {
	var taggedClusters []eksCluster
	region := *svc.Config.Region

	log.Debugf("Listing all EKS clusters in region %s", region)
	input := &eks.ListClustersInput{}
	result, err := svc.ListClusters(input)
	if err != nil {
		return nil, err
	}

	if len(result.Clusters) == 0 {
		log.Debugf("No EKS clusters were found in region %s", region)
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

		if ttlValue, ok := clusterInfo.Cluster.Tags[tagName]; ok {
			ttl, err := strconv.Atoi(*ttlValue)
			if err != nil {
				log.Errorf("Can't convert TTL tag for cluster %v (%s), may be the value is not correct", clusterName, region)
				continue
			}

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
				ClusterCreateTime:     *clusterInfo.Cluster.CreatedAt,
				ClusterNodeGroupsName: nodeGroups.Nodegroups,
				ClusterName:           clusterName,
				ClusterId:             clusterInfo.Cluster.Identity.String(),
				Status:                *clusterInfo.Cluster.Status,
				TTL:                   int64(ttl),
			})
		}
	}

	log.Debugf("Found %d EKS cluster(s) (%s) in ready status with ttl tag", len(taggedClusters), region)

	return taggedClusters, nil
}

//svc *eks.EKS, ec2Session *ec2.EC2, elbSession *elbv2.ELBV2, cloudwatchLogsSession *cloudwatch.CloudWatchLogs, cluster *eksCluster, tagName string, dryRun bool) error {

func deleteEKSCluster(cluster *eksCluster, sessions *AWSSessions, options *AwsOption) error {
	if cluster.Status == "DELETING" {
		log.Infof("EKS cluster %s (%s) is already in deletion process, skipping...", cluster.ClusterName, *sessions.EKS.Config.Region)
		return nil
	} else if cluster.Status == "CREATING" {
		log.Infof("EKS cluster %s (%s) is in creating process, skipping...", cluster.ClusterName, sessions.EKS.Config.Region)
		return nil
	} else {
		log.Infof("Deleting EKS cluster %s (%s), expired after %d seconds",
			cluster.ClusterName, *sessions.EKS.Config.Region, cluster.TTL)
	}

	if options.DryRun {
		return nil
	}

	// delete node groups
	if len(cluster.ClusterNodeGroupsName) > 0 {
		for _, nodeGroupName := range cluster.ClusterNodeGroupsName {
			nodeGroupStatus, _ := getNodeGroupStatus(sessions.EKS, cluster, *nodeGroupName)

			if nodeGroupStatus == "DELETING" {
				log.Infof("EKS cluster nodegroup %v (%s) is already in deletion process, skipping...", *nodeGroupName, cluster.ClusterName)
				continue
			} else if nodeGroupStatus == "CREATING" {
				log.Infof("EKS cluster nodegroup %v (%s) is in creating process, skipping...", *nodeGroupName, cluster.ClusterName)
				continue
			} else {
				log.Infof("Deleting EKS cluster nodegroup %v (%s)", *nodeGroupName, cluster.ClusterName)
			}

			err := deleteNodeGroupStatus(sessions.EKS, cluster, *nodeGroupName, options.DryRun)
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

	// tag associated load balancers for deletion
	lbsAssociatedToThisEksCluster, err := ListTaggedLoadBalancersWithKeyContains(sessions.ELB, cluster.ClusterName)
	if err != nil {
		return err
	}
	err = TagLoadBalancersForDeletion(sessions.ELB, options.TagName, lbsAssociatedToThisEksCluster)
	if err != nil {
		return err
	}

	// tag associated ebs for deletion
	err = TagVolumesFromEksClusterForDeletion(sessions.EC2, options.TagName, cluster.ClusterName)
	if err != nil {
		return err
	}

	// tag cloudwatch logs for deletion
	err = TagLogsForDeletion(sessions.CloudWatchLogs, options.TagName, cluster.ClusterId)
	if err != nil {
		return err
	}

	// add cluster creation date vpc for deletion
	err = TagVPCsForDeletion(sessions.EC2, options.TagName, cluster.ClusterName, cluster.ClusterCreateTime, cluster.TTL)
	if err != nil {
		return err
	}

	// delete EKS cluster
	_, err = sessions.EKS.DeleteCluster(
		&eks.DeleteClusterInput{
			Name: &cluster.ClusterName,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getNodeGroupStatus(svc *eks.EKS, cluster *eksCluster, nodeGroupName string) (string, error) {
	result, err := svc.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   &cluster.ClusterName,
		NodegroupName: &nodeGroupName,
	})
	if err != nil {
		return "", err
	}

	return *result.Nodegroup.Status, nil
}

func deleteNodeGroupStatus(svc *eks.EKS, cluster *eksCluster, nodeGroupName string, dryRun bool) error {
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

func DeleteExpiredEKSClusters(sessions *AWSSessions, options *AwsOption) error {
	clusters, err := listTaggedEKSClusters(sessions.EKS, options.TagName)
	if err != nil {
		return fmt.Errorf("can't list EKS clusters: %s\n", err)
	}

	for _, cluster := range clusters {
		if CheckIfExpired(cluster.ClusterCreateTime, cluster.TTL) {
			err := deleteEKSCluster(&cluster, sessions, options)
			if err != nil {
				log.Errorf("Deletion EKS cluster error %s/%s: %s",
					cluster.ClusterName, *sessions.EKS.Config.Region, err)
				continue
			}
		} else {
			log.Debugf("EKS cluster %s in %s, has not yet expired",
				cluster.ClusterName, *sessions.EKS.Config.Region)
		}
	}

	return nil
}
