package gcp

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/protobuf/proto"
)

func TestDeleteExpiredRouters(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dryRun=%t", dryRun), func(t *testing.T) {
			const base = "/compute/v1/projects/" + testProjectID + "/regions/europe-west1"
			listPages := 0
			deleted := make(map[string]bool)
			completed := make(map[string]bool)
			var mu sync.Mutex
			sessions := networkTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == base+"/routers":
					listPages++
					name, next := "router-one", "next"
					if r.URL.Query().Get("pageToken") == "next" {
						name, next = "router-two", ""
					}
					writeComputeResponse(t, w, &computepb.RouterList{
						NextPageToken: proto.String(next),
						Items: []*computepb.Router{
							{Name: proto.String(name), Description: proto.String(`{"ttl":3600,"creation_date":1}`)},
							{Name: proto.String("protected"), Description: proto.String(`{"ttl":0,"creation_date":1}`)},
						},
					})
				case r.Method == http.MethodDelete && (r.URL.Path == base+"/routers/router-one" || r.URL.Path == base+"/routers/router-two"):
					if dryRun {
						t.Error("router deletion in dry-run mode")
					}
					if listPages != 2 {
						t.Error("router deletion started before listing finished")
					}
					name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					if name == "router-two" && !completed["router-one"] {
						t.Error("router deletion did not wait for the preceding operation")
					}
					deleted[name] = true
					_, _ = fmt.Fprintf(w, `{"name":%q,"status":"PENDING"}`, name)
				case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, base+"/operations/"):
					name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					if !deleted[name] {
						t.Errorf("polled unknown deletion: %s", name)
					}
					completed[name] = true
					_, _ = fmt.Fprintf(w, `{"name":%q,"status":"DONE"}`, name)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			})
			DeleteExpiredRouters(sessions, GCPOptions{ProjectID: testProjectID, Location: "europe-west1", DryRun: dryRun})
			mu.Lock()
			defer mu.Unlock()
			for _, name := range []string{"router-one", "router-two"} {
				if completed[name] == dryRun {
					t.Errorf("deletion completed for %s = %v, want %v", name, completed[name], !dryRun)
				}
			}
		})
	}
}

func TestDeleteVPCBlockedByRouter(t *testing.T) {
	for _, failure := range []string{"list", "delete", "wait"} {
		t.Run(failure, func(t *testing.T) {
			const base = "/compute/v1/projects/" + testProjectID
			const networkURL = "https://www.googleapis.com" + base + "/global/networks/expired-network"
			var mu sync.Mutex
			failureReached := false
			sessions := networkTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == base+"/aggregated/routers":
					if failure == "list" {
						failureReached = true
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"error":{"code":403,"message":"router listing denied"}}`))
						return
					}
					writeComputeResponse(t, w, &computepb.RouterAggregatedList{
						Items: map[string]*computepb.RoutersScopedList{
							"regions/europe-west9": {Routers: []*computepb.Router{{
								Name: proto.String("blocking-router"), Network: proto.String(networkURL),
								Region: proto.String("https://www.googleapis.com" + base + "/regions/europe-west9"),
							}}},
						},
					})
				case r.Method == http.MethodDelete && r.URL.Path == base+"/regions/europe-west9/routers/blocking-router":
					if failure == "delete" {
						failureReached = true
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"error":{"code":400,"message":"router still in use"}}`))
						return
					}
					_, _ = w.Write([]byte(`{"name":"router-operation","status":"PENDING"}`))
				case r.Method == http.MethodGet && r.URL.Path == base+"/regions/europe-west9/operations/router-operation":
					failureReached = true
					_, _ = w.Write([]byte(`{"name":"router-operation","status":"DONE","httpErrorStatusCode":400,"error":{"errors":[{"code":"RESOURCE_IN_USE_BY_ANOTHER_RESOURCE","message":"router still in use"}]}}`))
				default:
					t.Errorf("cleanup continued with a blocked router: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
				}
			})
			deleteVPC(sessions, GCPOptions{ProjectID: testProjectID}, &computepb.Network{
				Name: proto.String("expired-network"), SelfLink: proto.String(networkURL),
				AutoCreateSubnetworks: proto.Bool(true),
			})
			mu.Lock()
			defer mu.Unlock()
			if !failureReached {
				t.Fatal("router failure was not exercised")
			}
		})
	}
}

func TestDeleteRouterAlreadyDeleted(t *testing.T) {
	sessions := networkTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/compute/v1/projects/"+testProjectID+"/regions/europe-west9/routers/router" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"router not found"}}`))
	})
	if err := deleteRouter(sessions, testProjectID, "europe-west9", "router"); err != nil {
		t.Fatalf("router already removed by the regional watcher: %s", err)
	}
}
