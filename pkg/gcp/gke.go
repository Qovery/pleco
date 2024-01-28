package gcp

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"fmt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"strconv"
	"strings"
	"time"
)

func DeleteExpiredGKEClusters(sessions GCPSessions, options GCPOptions) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	clustersIterator, err := sessions.Cluster.ListClusters(ctx, &containerpb.ListClustersRequest{Parent: fmt.Sprintf("projects/%s/locations/%s", options.ProjectID, options.Location)})
	if err != nil {
		log.Error(fmt.Sprintf("Error listing clusters, error: %s", err))
		return
	}

	for _, cluster := range clustersIterator.Clusters {
		ttlStr, ok := cluster.ResourceLabels["ttl"]
		if !ok || strings.TrimSpace(ttlStr) == "" {
			log.Info(fmt.Sprintf("No ttl label value found, ignoring this cluster (`%s`)", cluster.Name))
			continue
		}
		ttl, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("ttl label value `%s` is not parsable to int64, ignoring this cluster (`%s`)", ttlStr, cluster.Name))
			continue
		}
		creationTimeStr, ok := cluster.ResourceLabels["creation_date"]
		if !ok || strings.TrimSpace(ttlStr) == "" {
			log.Info(fmt.Sprintf("No creation_date label value found, ignoring this cluster (`%s`)", cluster.Name))
			continue
		}
		creationTimeInt64, err := strconv.ParseInt(creationTimeStr, 10, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("creation_date label value `%s` is not parsable to int64, ignoring this cluster (`%s`)", ttlStr, cluster.Name))
			continue
		}
		creationTime := time.Unix(creationTimeInt64, 0).UTC()
		// cluster is not expired (or is protected TTL = 0)
		if !ok || ttl == 0 || creationTimeInt64 == 0 || time.Now().UTC().Before(creationTime.Add(time.Second*time.Duration(ttl))) {
			continue
		}

		if options.DryRun {
			log.Info(fmt.Sprintf("Cluster `%s will be deleted`", cluster.Name))
			continue
		}

		// cluster is eligible to deletion
		log.Info(fmt.Sprintf("Deleting cluster `%s` created at `%s` UTC (TTL `{%d}` seconds)", cluster.Name, creationTime.UTC(), ttl))
		if _, err := sessions.Cluster.DeleteCluster(ctx, &containerpb.DeleteClusterRequest{
			Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", options.ProjectID, options.Location, cluster.Name),
		}); err != nil {
			log.Error(fmt.Sprintf("Error deleting cluster `%s`, error: %s", cluster.Name, err))
		}
	}
}
