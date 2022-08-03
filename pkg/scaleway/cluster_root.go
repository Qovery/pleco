package scaleway

import (
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/k8s/v1"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewayCluster struct {
	common.CloudProviderResource
	Name string
}

func DeleteExpiredClusters(sessions ScalewaySessions, options ScalewayOptions) {
	expiredClusters, _ := getExpiredClusters(sessions.Cluster, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired cluster", len(expiredClusters), options.Zone, true)

	log.Info(count)

	if options.DryRun || len(expiredClusters) == 0 {
		return
	}

	log.Info(start)

	for _, expiredCluster := range expiredClusters {
		deleteCluster(sessions.Cluster, expiredCluster, options.Zone)
	}
}

func ListClusters(clusterAPI *k8s.API, tagName string) ([]ScalewayCluster, string) {
	input := &k8s.ListClustersRequest{}
	result, err := clusterAPI.ListClusters(input)

	if err != nil {
		log.Errorf("Can't list cluster for region %s: %s", input.Region, err.Error())
		return []ScalewayCluster{}, input.Region.String()
	}

	clusters := []ScalewayCluster{}
	for _, cluster := range result.Clusters {
		essentialTags := common.GetEssentialTags(cluster.Tags, tagName)
		creationDate, _ := time.Parse(time.RFC3339, cluster.CreatedAt.Format(time.RFC3339))

		clusters = append(clusters, ScalewayCluster{
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

	return clusters, input.Region.String()
}

func getExpiredClusters(clusterAPI *k8s.API, options *ScalewayOptions) ([]ScalewayCluster, string) {
	clusters, region := ListClusters(clusterAPI, options.TagName)

	expiredClusters := []ScalewayCluster{}
	for _, cluster := range clusters {
		if cluster.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredClusters = append(expiredClusters, cluster)
		}
	}

	return expiredClusters, region
}

func deleteCluster(clusterAPI *k8s.API, cluster ScalewayCluster, region string) {
	_, err := clusterAPI.DeleteCluster(
		&k8s.DeleteClusterRequest{
			ClusterID:               cluster.Identifier,
			WithAdditionalResources: true,
		})

	if err != nil {
		log.Errorf("Can't delete cluster %s: %s", cluster.Name, err.Error())
	} else {
		log.Debugf("Kapsule cluster %s in %s deleted.", cluster.Name, region)
	}
}
