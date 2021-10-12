package do

import (
	"context"
	"github.com/Qovery/pleco/pkg/common"
	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"
	"time"
)

type DOCluster struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredClusters(sessions *DOSessions, options *DOOptions) {
	expiredClusters, region := getExpiredClusters(sessions.Client, options.TagName, options.Region)

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
			ID:           cluster.ID,
			Name:         cluster.Name,
			CreationDate: creationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return clusters
}

func getExpiredClusters(client *godo.Client, tagName string, region string) ([]DOCluster, string) {
	clusters := listClusters(client, tagName, region)

	expiredClusters := []DOCluster{}
	for _, cluster := range clusters {
		if common.CheckIfExpired(cluster.CreationDate, cluster.TTL, "cluster"+cluster.Name) && !cluster.IsProtected {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	return expiredClusters, region
}

func deleteCluster(client *godo.Client, cluster DOCluster) {
	_, err := client.Kubernetes.Delete(context.TODO(), cluster.ID)

	if err != nil {
		log.Errorf("Can't delete cluster %s: %s", cluster.Name, err.Error())
	}
}
