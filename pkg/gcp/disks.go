package gcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/container/apiv1/containerpb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	diskClusterNameLabel     = "goog-k8s-cluster-name"
	diskClusterLocationLabel = "goog-k8s-cluster-location"
)

type scopedDisk struct {
	disk  *computepb.Disk
	scope string
}

// DeleteOrphanedDisks cleans up disks left behind after GKE cluster deletion.
// Detachment alone is insufficient: a scaled-down workload may still own a PVC.
func DeleteOrphanedDisks(sessions GCPSessions, options GCPOptions) {
	disks, err := listOrphanDiskCandidates(sessions, options)
	if err != nil {
		log.Errorf("Error listing disks in region %s: %s", options.Location, err)
		return
	}
	for _, disk := range disks {
		if err := deleteOrphanedDisk(sessions, options, disk); err != nil {
			log.Errorf("Error cleaning up disk %s/%s: %s", disk.scope, disk.disk.GetName(), err)
		}
	}
}

func listOrphanDiskCandidates(sessions GCPSessions, options GCPOptions) ([]scopedDisk, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	it := sessions.Disk.AggregatedList(ctx, &computepb.AggregatedListDisksRequest{
		Project:          options.ProjectID,
		IncludeAllScopes: proto.Bool(true), // Include regional as well as zonal disks.
	})
	var disks []scopedDisk
	for {
		entry, err := it.Next()
		if errors.Is(err, iterator.Done) {
			return disks, nil
		}
		if err != nil {
			return nil, err
		}
		if !diskScopeInRegion(entry.Key, options.Location) {
			continue
		}
		for _, disk := range entry.Value.GetDisks() {
			if diskCleanupCandidate(disk, options) {
				disks = append(disks, scopedDisk{disk: disk, scope: entry.Key})
			}
		}
	}
}

func diskScopeInRegion(scope, region string) bool {
	if region == "" {
		return false
	}
	if scope == "regions/"+region {
		return true
	}
	zone, ok := strings.CutPrefix(scope, "zones/")
	if !ok {
		return false
	}
	separator := strings.LastIndex(zone, "-")
	return separator > 0 && separator < len(zone)-1 && zone[:separator] == region
}

func diskCleanupCandidate(disk *computepb.Disk, options GCPOptions) bool {
	if disk.GetName() == "" || disk.GetStatus() != "READY" || len(disk.GetUsers()) != 0 {
		return false
	}
	labels := disk.GetLabels()
	if strings.TrimSpace(labels[diskClusterNameLabel]) == "" || strings.TrimSpace(labels[diskClusterLocationLabel]) == "" {
		log.Debugf("Skipping disk %s: missing GKE cluster ownership labels", disk.GetName())
		return false
	}
	if value, ok := labels["do_not_delete"]; ok {
		protected, err := strconv.ParseBool(value)
		if err != nil || protected {
			return false
		}
	}
	if value, ok := labels["ttl"]; ok {
		ttl, err := strconv.ParseInt(value, 10, 64)
		if err != nil || ttl <= 0 {
			return false
		}
	}
	if options.IsDestroyingCommand || strings.TrimSpace(options.TagValue) != "" {
		return options.TagName != "" && options.TagValue != "" && strings.EqualFold(labels[options.TagName], options.TagValue)
	}
	return true
}

func deleteOrphanedDisk(sessions GCPSessions, options GCPOptions, candidate scopedDisk) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Re-read attachments, ownership and protection labels before making a decision.
	// Listing is completed first so deleting disks cannot disturb pagination.
	location := strings.SplitN(candidate.scope, "/", 2)[1]
	regional := strings.HasPrefix(candidate.scope, "regions/")
	var disk *computepb.Disk
	var err error
	if regional {
		disk, err = sessions.RegionDisk.Get(ctx, &computepb.GetRegionDiskRequest{
			Project: options.ProjectID, Region: location, Disk: candidate.disk.GetName(),
		})
	} else {
		disk, err = sessions.Disk.Get(ctx, &computepb.GetDiskRequest{
			Project: options.ProjectID, Zone: location, Disk: candidate.disk.GetName(),
		})
	}
	if err != nil {
		return fmt.Errorf("get disk: %w", err)
	}
	if disk.GetId() != candidate.disk.GetId() || !diskCleanupCandidate(disk, options) {
		return nil
	}

	// Look up the exact owner, including zonal clusters and clusters outside the
	// disk's region. Only NotFound proves it is gone; permission errors do not.
	clusterName := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", options.ProjectID,
		disk.Labels[diskClusterLocationLabel], disk.Labels[diskClusterNameLabel])
	_, err = sessions.Cluster.GetCluster(ctx, &containerpb.GetClusterRequest{Name: clusterName})
	if err == nil {
		log.Debugf("Skipping disk %s: cluster %s still exists", disk.GetName(), clusterName)
		return nil
	}
	var apiErr *googleapi.Error
	notFound := status.Code(err) == codes.NotFound || (errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound)
	if !notFound {
		return fmt.Errorf("check owning cluster %s: %w", clusterName, err)
	}

	if options.DryRun {
		log.Infof("Disk %s/%s would be deleted: detached and cluster %s no longer exists", candidate.scope, disk.GetName(), clusterName)
		return nil
	}
	log.Infof("Deleting orphaned disk %s/%s (cluster %s no longer exists)", candidate.scope, disk.GetName(), clusterName)
	var operation *compute.Operation
	if regional {
		operation, err = sessions.RegionDisk.Delete(ctx, &computepb.DeleteRegionDiskRequest{
			Project: options.ProjectID, Region: location, Disk: disk.GetName(),
		})
	} else {
		operation, err = sessions.Disk.Delete(ctx, &computepb.DeleteDiskRequest{
			Project: options.ProjectID, Zone: location, Disk: disk.GetName(),
		})
	}
	if err != nil {
		return fmt.Errorf("delete disk: %w", err)
	}
	if err := operation.Wait(ctx); err != nil {
		return fmt.Errorf("wait for disk deletion: %w", err)
	}
	return nil
}
