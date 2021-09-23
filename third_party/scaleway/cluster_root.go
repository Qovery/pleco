package scaleway

import (
	"github.com/Qovery/pleco/utils"
	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayCluster struct {
	ID string
	Name string
	CreationDate time.Time
	TTL int64
	IsProtected bool
}

func DeleteExpiredClusters(sessions *ScalewaySessions, options *ScalewayOption) {
	expiredClusters, region := getExpiredClusters(sessions.Cluster, options.TagName)

	count, start := utils.ElemToDeleteFormattedInfos("expired cluster", len(expiredClusters), region)

	log.Debug(count)

	if options.DryRun || len(expiredClusters) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredCluster := range expiredClusters {
		deleteCluster(sessions.Cluster, expiredCluster)
	}
}

func listClusters(clusterAPI *k8s.API, tagName string) ([]ScalewayCluster, string) {
	input := &k8s.ListClustersRequest{
		Status: "ready",
	}
	result, err := clusterAPI.ListClusters(input)

	if err != nil {
		log.Errorf("Can't list cluster for region %s: %s", input.Region, err.Error())
	}

	var clusters []ScalewayCluster
	for _ , cluster := range result.Clusters {
		_, ttl, isProtected, _, _ := utils.GetEssentialTags(cluster.Tags, tagName)
		creationDate, _ := time.Parse(time.RFC3339, cluster.CreatedAt.Format(time.RFC3339))

		clusters = append(clusters, ScalewayCluster{
			ID:           cluster.ID,
			Name:         cluster.Name,
			CreationDate: creationDate,
			TTL:          ttl,
			IsProtected:  isProtected,
		})
	}


	return clusters, input.Region.String()
}

func getExpiredClusters(clusterAPI *k8s.API, tagName string) ([]ScalewayCluster, string) {
	clusters, region := listClusters(clusterAPI, tagName)

	expiredClusters := []ScalewayCluster{}
	for _, cluster := range clusters {
		if utils.CheckIfExpired(cluster.CreationDate, cluster.TTL, "cluster" + cluster.Name) {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	return expiredClusters, region
}

func deleteCluster(clusterAPI *k8s.API, cluster ScalewayCluster) {
	_, err := clusterAPI.DeleteCluster(
		&k8s.DeleteClusterRequest{
			ClusterID: cluster.ID,
		})

	if err != nil {
		log.Errorf("Can't delete cluster %s", cluster.Name)
	}
}
