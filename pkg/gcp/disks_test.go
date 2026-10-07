package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	container "cloud.google.com/go/container/apiv1"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func orphanDisk() *computepb.Disk {
	return &computepb.Disk{
		Id: proto.Uint64(123), Name: proto.String("pvc-test"), Status: proto.String("READY"),
		Labels: map[string]string{
			diskClusterNameLabel: "deleted-cluster", diskClusterLocationLabel: "europe-west9-a",
		},
	}
}

func diskTestSessions(t *testing.T, handler http.HandlerFunc) GCPSessions {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	opts := []option.ClientOption{option.WithEndpoint(server.URL), option.WithoutAuthentication()}
	disks, err := compute.NewDisksRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = disks.Close() })
	regionalDisks, err := compute.NewRegionDisksRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = regionalDisks.Close() })
	clusters, err := container.NewClusterManagerRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clusters.Close() })
	return GCPSessions{Disk: disks, RegionDisk: regionalDisks, Cluster: clusters}
}

func TestDiskCleanupCandidate(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*computepb.Disk)
		opts   GCPOptions
		want   bool
	}{
		{name: "orphan without TTL", want: true},
		{name: "attached to any VM", modify: func(d *computepb.Disk) { d.Users = []string{"instances/other-cluster-node"} }},
		{name: "creating", modify: func(d *computepb.Disk) { d.Status = proto.String("CREATING") }},
		{name: "missing name", modify: func(d *computepb.Disk) { d.Name = nil }},
		{name: "missing owner", modify: func(d *computepb.Disk) { delete(d.Labels, diskClusterNameLabel) }},
		{name: "missing owner location", modify: func(d *computepb.Disk) { delete(d.Labels, diskClusterLocationLabel) }},
		{name: "unlabeled disk", modify: func(d *computepb.Disk) { d.Labels = nil }},
		{name: "protected", modify: func(d *computepb.Disk) { d.Labels["do_not_delete"] = "true" }},
		{name: "invalid protection", modify: func(d *computepb.Disk) { d.Labels["do_not_delete"] = "invalid" }},
		{name: "explicitly unprotected", modify: func(d *computepb.Disk) { d.Labels["do_not_delete"] = "false" }, want: true},
		{name: "zero TTL", modify: func(d *computepb.Disk) { d.Labels["ttl"] = "0" }},
		{name: "invalid TTL", modify: func(d *computepb.Disk) { d.Labels["ttl"] = "invalid" }},
		{name: "positive TTL does not delay orphan cleanup", modify: func(d *computepb.Disk) { d.Labels["ttl"] = "3600" }, want: true},
		{name: "destroy tag mismatch", opts: GCPOptions{IsDestroyingCommand: true, TagName: "owner", TagValue: "test"}},
		{name: "destroy tag match", modify: func(d *computepb.Disk) { d.Labels["owner"] = "test" }, opts: GCPOptions{IsDestroyingCommand: true, TagName: "owner", TagValue: "test"}, want: true},
		{name: "destroy without selector", opts: GCPOptions{IsDestroyingCommand: true}},
		{name: "disabling TTL checks does not bypass protection", modify: func(d *computepb.Disk) { d.Labels["ttl"] = "0" }, opts: GCPOptions{DisableTTLCheck: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disk := orphanDisk()
			if tt.modify != nil {
				tt.modify(disk)
			}
			if got := diskCleanupCandidate(disk, tt.opts); got != tt.want {
				t.Fatalf("diskCleanupCandidate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteOrphanedDisk(t *testing.T) {
	tests := []struct {
		name           string
		regional       bool
		dryRun         bool
		clusterStatus  int
		diskStatus     int
		deleteStatus   int
		operationError bool
		modify         func(*computepb.Disk)
		wantDelete     bool
		wantError      bool
	}{
		{name: "zonal orphan", clusterStatus: 404, wantDelete: true},
		{name: "regional orphan", regional: true, clusterStatus: 404, wantDelete: true},
		{name: "live owner", clusterStatus: 200},
		{name: "regional cluster owner", modify: func(d *computepb.Disk) { d.Labels[diskClusterLocationLabel] = "europe-west9" }, clusterStatus: 200},
		{name: "owner outside disk region", modify: func(d *computepb.Disk) { d.Labels[diskClusterLocationLabel] = "us-central1-a" }, clusterStatus: 200},
		{name: "dry run", dryRun: true, clusterStatus: 404},
		{name: "cluster lookup forbidden", clusterStatus: 403, wantError: true},
		{name: "cluster lookup bad request", clusterStatus: 400, wantError: true},
		{name: "disk read failure", diskStatus: 403, wantError: true},
		{name: "delete failure", clusterStatus: 404, deleteStatus: 403, wantDelete: true, wantError: true},
		{name: "asynchronous delete failure", clusterStatus: 404, operationError: true, wantDelete: true, wantError: true},
		{name: "attached since listing", modify: func(d *computepb.Disk) { d.Users = []string{"instances/node"} }},
		{name: "protected since listing", modify: func(d *computepb.Disk) { d.Labels["do_not_delete"] = "true" }},
		{name: "replaced since listing", modify: func(d *computepb.Disk) { d.Id = proto.Uint64(456) }},
		{name: "ownership removed since listing", modify: func(d *computepb.Disk) { d.Labels = nil }},
		{name: "new owner since listing", modify: func(d *computepb.Disk) { d.Labels[diskClusterNameLabel] = "new-owner" }, clusterStatus: 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disk := orphanDisk()
			fresh := proto.Clone(disk).(*computepb.Disk)
			if tt.modify != nil {
				tt.modify(fresh)
			}
			scope := "zones/europe-west9-a"
			if tt.regional {
				scope = "regions/europe-west9"
			}
			diskPath := "/compute/v1/projects/" + testProjectID + "/" + scope + "/disks/pvc-test"
			clusterPath := "/v1/projects/" + testProjectID + "/locations/" + fresh.Labels[diskClusterLocationLabel] + "/clusters/" + fresh.Labels[diskClusterNameLabel]
			deleted := false
			sessions := diskTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/compute/v1/projects/"+testProjectID+"/"+scope+"/operations/delete-disk":
					if tt.operationError {
						_, _ = fmt.Fprint(w, `{"name":"delete-disk","status":"DONE","httpErrorStatusCode":400,"httpErrorMessage":"disk attached","error":{"errors":[{"code":"RESOURCE_IN_USE_BY_ANOTHER_RESOURCE","message":"disk attached"}]}}`)
					} else {
						_, _ = fmt.Fprint(w, `{"name":"delete-disk","status":"DONE"}`)
					}
				case r.Method == http.MethodGet && r.URL.Path == diskPath:
					if tt.diskStatus != 0 {
						w.WriteHeader(tt.diskStatus)
						_, _ = fmt.Fprint(w, `{"error":{"message":"cannot read disk"}}`)
						return
					}
					body, _ := protojson.Marshal(fresh)
					_, _ = w.Write(body)
				case r.Method == http.MethodGet && r.URL.Path == clusterPath:
					if tt.clusterStatus == 0 {
						t.Error("unexpected cluster lookup for an ineligible disk")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.WriteHeader(tt.clusterStatus)
					if tt.clusterStatus == 200 {
						// Even a deleting cluster still protects its disks.
						_, _ = fmt.Fprint(w, `{"name":"owner","status":"STOPPING"}`)
					} else {
						_, _ = fmt.Fprint(w, `{"error":{"message":"cluster lookup failed"}}`)
					}
				case r.Method == http.MethodDelete && r.URL.Path == diskPath:
					deleted = true
					if tt.deleteStatus != 0 {
						w.WriteHeader(tt.deleteStatus)
						_, _ = fmt.Fprint(w, `{"error":{"message":"cannot delete disk"}}`)
					} else {
						_, _ = fmt.Fprint(w, `{"name":"delete-disk","status":"DONE"}`)
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			})
			err := deleteOrphanedDisk(sessions, GCPOptions{ProjectID: testProjectID, DryRun: tt.dryRun}, scopedDisk{disk: disk, scope: scope})
			if (err != nil) != tt.wantError {
				t.Errorf("error = %v, wantError %v", err, tt.wantError)
			}
			if deleted != tt.wantDelete {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDelete)
			}
		})
	}
}

func TestDeleteOrphanedDisksPaginationAndRegionFiltering(t *testing.T) {
	for _, failSecondPage := range []bool{false, true} {
		t.Run(fmt.Sprintf("listing_failure_%v", failSecondPage), func(t *testing.T) {
			var deleted []string
			pages := 0
			sessions := diskTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/aggregated/disks"):
					pages++
					if r.URL.Query().Get("includeAllScopes") != "true" {
						t.Error("regional disks must be included")
					}
					if r.URL.Query().Get("pageToken") == "next" && failSecondPage {
						w.WriteHeader(http.StatusForbidden)
						_, _ = fmt.Fprint(w, `{"error":{"message":"listing failed"}}`)
						return
					}
					body, _ := protojson.Marshal(orphanDisk())
					items := map[string]any{}
					page := map[string]any{"items": items}
					if r.URL.Query().Get("pageToken") == "" {
						items["zones/europe-west9-a"] = map[string]any{"disks": []json.RawMessage{body}}
						page["nextPageToken"] = "next"
					} else {
						items["regions/europe-west9"] = map[string]any{"disks": []json.RawMessage{body}}
						items["zones/europe-west90-a"] = map[string]any{"disks": []json.RawMessage{body}}
						items["regions/us-central1"] = map[string]any{"disks": []json.RawMessage{body}}
						items["zones/europe-west9-b"] = map[string]any{"warning": map[string]string{"code": "NO_RESULTS_ON_PAGE"}}
					}
					_ = json.NewEncoder(w).Encode(page)
				case strings.HasPrefix(r.URL.Path, "/v1/projects/"):
					w.WriteHeader(http.StatusNotFound)
					_, _ = fmt.Fprint(w, `{"error":{"message":"cluster deleted"}}`)
				case strings.Contains(r.URL.Path, "/operations/"):
					_, _ = fmt.Fprint(w, `{"name":"delete-disk","status":"DONE"}`)
				case r.Method == http.MethodGet:
					body, _ := protojson.Marshal(orphanDisk())
					_, _ = w.Write(body)
				case r.Method == http.MethodDelete:
					if pages != 2 {
						t.Error("deletion started before listing completed")
					}
					deleted = append(deleted, r.URL.Path)
					if strings.Contains(r.URL.Path, "/zones/") {
						// One failed deletion must not prevent other orphan cleanup.
						w.WriteHeader(http.StatusForbidden)
						_, _ = fmt.Fprint(w, `{"error":{"message":"delete failed"}}`)
					} else {
						_, _ = fmt.Fprint(w, `{"name":"delete-disk","status":"DONE"}`)
					}
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			})
			DeleteOrphanedDisks(sessions, GCPOptions{ProjectID: testProjectID, Location: "europe-west9"})
			var want []string
			if !failSecondPage {
				want = []string{
					"/compute/v1/projects/" + testProjectID + "/zones/europe-west9-a/disks/pvc-test",
					"/compute/v1/projects/" + testProjectID + "/regions/europe-west9/disks/pvc-test",
				}
			}
			if !reflect.DeepEqual(deleted, want) {
				t.Errorf("delete requests = %v, want %v", deleted, want)
			}
		})
	}
}
