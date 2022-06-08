package do

import (
	"context"
	"time"

	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type DOCluster struct {
	common.CloudProviderResource
	Name string
}

func DeleteExpiredClusters(sessions DOSessions, options DOOptions) {
	expiredClusters, region := getExpiredClusters(sessions.Client, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired cluster", len(expiredClusters), region)

	log.Debug(count)

	if options.DryRun || len(expiredClusters) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredCluster := range expiredClusters {
		deleteCluster(sessions.Client, expiredCluster)
	}
}

func listClusters(client *godo.Client, tagName string, region string) []DOCluster {
	result, _, err := client.Kubernetes.List(context.TODO(), &godo.ListOptions{})

	if err != nil {
		log.Errorf("Can't list cluster for region %s: %s", region, err.Error())
		return []DOCluster{}
	}

	clusters := []DOCluster{}
	for _, cluster := range result {
		essentialTags := common.GetEssentialTags(cluster.Tags, tagName)
		creationDate, _ := time.Parse(time.RFC3339, cluster.CreatedAt.Format(time.RFC3339))

		clusters = append(clusters, DOCluster{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   cluster.ID,
				Description:  "Cluster: " + cluster.Name,
				CreationDate: creationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Name: cluster.Name,
		})
	}

	return clusters
}

func getExpiredClusters(client *godo.Client, options *DOOptions) ([]DOCluster, string) {
	clusters := listClusters(client, options.TagName, options.Region)

	expiredClusters := []DOCluster{}
	for _, cluster := range clusters {
		if cluster.IsResourceExpired(options.TagValue) {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	return expiredClusters, options.Region
}

func deleteCluster(client *godo.Client, cluster DOCluster) {
	_, err := client.Kubernetes.Delete(context.TODO(), cluster.Identifier)

	if err != nil {
		log.Errorf("Can't delete cluster %s: %s", cluster.Name, err.Error())
	}
}
