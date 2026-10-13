package gcp

import (
	"fmt"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// GKECluster expires from its ttl and creation_date resource labels.
type GKECluster struct {
	*containerpb.Cluster
}

func (c GKECluster) IsExpired() bool {
	return c.Cluster != nil && isExpiredFromUnixCreation("cluster", c.Name, c.ResourceLabels["ttl"], c.ResourceLabels["creation_date"], false)
}

func DeleteExpiredGKEClusters(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	clustersIterator, err := sessions.Cluster.ListClusters(ctx, &containerpb.ListClustersRequest{Parent: fmt.Sprintf("projects/%s/locations/%s", options.ProjectID, options.Location)})
	if err != nil {
		log.Error(fmt.Sprintf("Error listing clusters, error: %s", err))
		return
	}

	for _, cluster := range clustersIterator.Clusters {
		if !(GKECluster{Cluster: cluster}).IsExpired() {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Cluster `%s will be deleted`", cluster.Name))
			continue
		}

		// cluster is eligible to deletion
		log.Infof("Deleting cluster `%s` (creation_date `%s`, TTL `%s` seconds)", cluster.Name, cluster.ResourceLabels["creation_date"], cluster.ResourceLabels["ttl"])
		if _, err := sessions.Cluster.DeleteCluster(ctx, &containerpb.DeleteClusterRequest{
			Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", options.ProjectID, options.Location, cluster.Name),
		}); err != nil {
			log.Error(fmt.Sprintf("Error deleting cluster `%s`, error: %s", cluster.Name, err))
		}
	}
}
