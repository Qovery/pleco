package gcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func networkTestSessions(t *testing.T, handler http.HandlerFunc) GCPSessions {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	opts := []option.ClientOption{option.WithEndpoint(server.URL), option.WithoutAuthentication()}
	networks, err := compute.NewNetworksRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = networks.Close() })
	subnets, err := compute.NewSubnetworksRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = subnets.Close() })
	routes, err := compute.NewRoutesRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = routes.Close() })
	firewalls, err := compute.NewFirewallsRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = firewalls.Close() })
	routers, err := compute.NewRoutersRESTClient(context.Background(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = routers.Close() })
	return GCPSessions{Network: networks, Subnetwork: subnets, Route: routes, Firewall: firewalls, Router: routers}
}

func writeComputeResponse(t *testing.T, w http.ResponseWriter, response proto.Message) {
	t.Helper()
	body, err := protojson.Marshal(response)
	if err != nil {
		t.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

func TestDeleteExpiredVPCs(t *testing.T) {
	const expired = `{"ttl":3600,"creation_date":1}`
	tests := []struct {
		name        string
		autoMode    bool
		description string
		dryRun      bool
		wantDelete  bool
	}{
		{name: "auto mode", autoMode: true, description: expired, wantDelete: true},
		{name: "custom mode", description: expired, wantDelete: true},
		{name: "auto dry run", autoMode: true, description: expired, dryRun: true},
		{name: "custom dry run", description: expired, dryRun: true},
		{name: "missing TTL", description: `{"creation_date":1}`},
		{name: "zero TTL", description: `{"ttl":0,"creation_date":1}`},
		{name: "invalid TTL", description: `{"ttl":"invalid","creation_date":1}`},
		{name: "missing creation date", description: `{"ttl":3600}`},
		{name: "zero creation date", description: `{"ttl":3600,"creation_date":0}`},
		{name: "not expired", description: fmt.Sprintf(`{"ttl":86400,"creation_date":%d}`, time.Now().Unix())},
		{name: "invalid description", description: "not JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const base = "/compute/v1/projects/" + testProjectID
			networkName := "expired-network"
			networkPath := base + "/global/networks/" + networkName
			networkURL := "https://www.googleapis.com" + networkPath
			completed := make(map[string]bool)
			deleted := make(map[string]bool)
			subnetPages := 0
			routerPages := 0
			var mu sync.Mutex
			sessions := networkTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && r.URL.Path == base+"/global/networks" {
					writeComputeResponse(t, w, &computepb.NetworkList{Items: []*computepb.Network{{
						Name: proto.String(networkName), SelfLink: proto.String(networkURL),
						AutoCreateSubnetworks: proto.Bool(tt.autoMode), Description: proto.String(tt.description),
					}}})
					return
				}
				if !tt.wantDelete {
					t.Errorf("ineligible network triggered a request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switch {
				case r.Method == http.MethodGet && (r.URL.Path == base+"/global/routes" || r.URL.Path == base+"/global/firewalls" || r.URL.Path == base+"/aggregated/subnetworks" || r.URL.Path == base+"/aggregated/routers"):
					if got := r.URL.Query().Get("filter"); got != fmt.Sprintf(`network = "%s"`, networkURL) {
						t.Errorf("unexpected network filter: %s", got)
					}
					switch r.URL.Path {
					case base + "/aggregated/routers":
						routerPages++
						region, name, next := "europe-west9", "router-west", "next"
						if r.URL.Query().Get("pageToken") == "next" {
							region, name, next = "us-east1", "router-east", ""
						}
						writeComputeResponse(t, w, &computepb.RouterAggregatedList{
							NextPageToken: proto.String(next),
							Items: map[string]*computepb.RoutersScopedList{
								"regions/" + region: {Routers: []*computepb.Router{
									{Name: proto.String(name), Network: proto.String(networkURL),
										Region: proto.String("https://www.googleapis.com" + base + "/regions/" + region)},
									{Name: proto.String("other-router"), Network: proto.String("other-network")},
								}},
							},
						})
					case base + "/global/routes":
						writeComputeResponse(t, w, &computepb.RouteList{Items: []*computepb.Route{
							{Name: proto.String("static-route"), Network: proto.String(networkURL), RouteType: proto.String("STATIC")},
							{Name: proto.String("subnet-route"), Network: proto.String(networkURL), RouteType: proto.String("SUBNET")},
							{Name: proto.String("legacy-subnet-route"), Network: proto.String(networkURL), NextHopNetwork: proto.String(networkURL)},
							{Name: proto.String("dynamic-route"), Network: proto.String(networkURL), RouteType: proto.String("BGP"), NextHopIp: proto.String("10.0.0.1")},
							{Name: proto.String("peering-route"), Network: proto.String(networkURL), RouteType: proto.String("STATIC"), NextHopPeering: proto.String("peer")},
							{Name: proto.String("default-route-123"), Network: proto.String(networkURL), RouteType: proto.String("STATIC"),
								DestRange: proto.String("0.0.0.0/0"), Priority: proto.Uint32(1000),
								NextHopGateway: proto.String("https://www.googleapis.com" + base + "/global/gateways/default-internet-gateway")},
							{Name: proto.String("other-route"), Network: proto.String("other-network"), RouteType: proto.String("STATIC")},
						}})
					case base + "/global/firewalls":
						writeComputeResponse(t, w, &computepb.FirewallList{Items: []*computepb.Firewall{
							{Name: proto.String("firewall"), Network: proto.String(networkURL)},
							{Name: proto.String("other-firewall"), Network: proto.String("other-network")},
						}})
					case base + "/aggregated/subnetworks":
						if tt.autoMode {
							t.Error("auto-mode subnets must be left to network deletion")
						}
						subnetPages++
						region, name, next := "europe-west1", "subnet-west", "next"
						if r.URL.Query().Get("pageToken") == "next" {
							region, name, next = "us-east1", "subnet-east", ""
						}
						writeComputeResponse(t, w, &computepb.SubnetworkAggregatedList{
							NextPageToken: proto.String(next),
							Items: map[string]*computepb.SubnetworksScopedList{
								"regions/" + region: {Subnetworks: []*computepb.Subnetwork{{
									Name: proto.String(name), Network: proto.String(networkURL),
									Region: proto.String("https://www.googleapis.com" + base + "/regions/" + region),
								}}},
							},
						})
					}
				case r.Method == http.MethodDelete:
					name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					switch r.URL.Path {
					case base + "/regions/europe-west9/routers/router-west", base + "/regions/us-east1/routers/router-east":
						if routerPages != 2 {
							t.Error("router deletion started before listing finished")
						}
					case base + "/global/routes/static-route", base + "/global/firewalls/firewall":
					case base + "/regions/europe-west1/subnetworks/subnet-west", base + "/regions/us-east1/subnetworks/subnet-east":
						if !completed["router-west"] || !completed["router-east"] {
							t.Error("subnet deletion before router operations completed")
						}
						if tt.autoMode {
							t.Error("auto-mode subnet deletion attempted")
						}
					case networkPath:
						dependencies := []string{"static-route", "firewall", "router-west", "router-east"}
						if !tt.autoMode {
							dependencies = append(dependencies, "subnet-west", "subnet-east")
						}
						for _, dependency := range dependencies {
							if !completed[dependency] {
								t.Errorf("network deletion before dependency %s finished deleting", dependency)
							}
						}
					default:
						t.Errorf("unexpected deletion: %s", r.URL)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					deleted[name] = true
					_, _ = fmt.Fprintf(w, `{"name":%q,"status":"PENDING"}`, name)
				case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/operations/"):
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
			DeleteExpiredVPCs(sessions, GCPOptions{ProjectID: testProjectID, DryRun: tt.dryRun})
			mu.Lock()
			defer mu.Unlock()
			if completed[networkName] != tt.wantDelete {
				t.Errorf("network deletion completed = %v, want %v", completed[networkName], tt.wantDelete)
			}
			if tt.wantDelete && !tt.autoMode && subnetPages != 2 {
				t.Errorf("subnet pages = %d, want 2", subnetPages)
			}
		})
	}
}

func TestDeleteExpiredVPCsWorkerLimit(t *testing.T) {
	const base = "/compute/v1/projects/" + testProjectID
	const networkCount = 21
	started := make(chan string, networkCount)
	firstBatchReady := make(chan string, networkCount)
	lastReady := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	releaseLast := make(chan struct{})
	var releaseFirstOnce, releaseLastOnce sync.Once
	defer releaseFirstOnce.Do(func() { close(releaseFirst) })
	defer releaseLastOnce.Do(func() { close(releaseLast) })

	var mu sync.Mutex
	active, peak, completed, listPages := 0, 0, 0, 0
	seen := make(map[string]bool)
	sessions := networkTestSessions(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base+"/global/networks":
			mu.Lock()
			listPages++
			mu.Unlock()
			first, end, next := 0, 20, "next"
			if r.URL.Query().Get("pageToken") == "next" {
				first, end, next = 20, networkCount, ""
			}
			var networks []*computepb.Network
			for i := first; i < end; i++ {
				name := fmt.Sprintf("network-%02d", i)
				networks = append(networks, &computepb.Network{
					Name: proto.String(name), AutoCreateSubnetworks: proto.Bool(true),
					SelfLink:    proto.String("https://www.googleapis.com" + base + "/global/networks/" + name),
					Description: proto.String(`{"ttl":3600,"creation_date":1}`),
				})
			}
			writeComputeResponse(t, w, &computepb.NetworkList{Items: networks, NextPageToken: proto.String(next)})
		case r.Method == http.MethodGet && r.URL.Path == base+"/aggregated/routers":
			name := strings.TrimSuffix(r.URL.Query().Get("filter"), `"`)
			name = name[strings.LastIndex(name, "/")+1:]
			mu.Lock()
			if listPages != 2 {
				t.Error("network deletion started before listing finished")
			}
			active++
			if active > peak {
				peak = active
			}
			mu.Unlock()
			started <- name
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && (r.URL.Path == base+"/global/routes" || r.URL.Path == base+"/global/firewalls"):
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, base+"/global/networks/"):
			name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			mu.Lock()
			if seen[name] {
				t.Errorf("duplicate network deletion: %s", name)
			}
			seen[name] = true
			mu.Unlock()
			_, _ = fmt.Fprintf(w, `{"name":%q,"status":"PENDING"}`, name)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, base+"/global/operations/"):
			name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			gate := releaseFirst
			if name == "network-20" {
				lastReady <- struct{}{}
				gate = releaseLast
			} else {
				firstBatchReady <- name
			}
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			}
			mu.Lock()
			active--
			completed++
			mu.Unlock()
			_, _ = fmt.Fprintf(w, `{"name":%q,"status":"DONE"}`, name)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	done := make(chan struct{})
	go func() {
		DeleteExpiredVPCs(sessions, GCPOptions{ProjectID: testProjectID})
		close(done)
	}()
	// On failure, unblock the HTTP handlers and drain the worker goroutines.
	defer func() {
		releaseFirstOnce.Do(func() { close(releaseFirst) })
		releaseLastOnce.Do(func() { close(releaseLast) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("network workers failed to stop")
		}
	}()
	for i := 0; i < 20; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("20 networks did not start concurrently")
		}
		select {
		case <-firstBatchReady:
		case <-time.After(5 * time.Second):
			t.Fatal("network workers did not reach the operation wait")
		}
	}
	select {
	case name := <-started:
		t.Fatalf("network %s exceeded the 20-worker limit while operations were pending", name)
	case <-done:
		t.Fatal("cleanup returned while operations were pending")
	case <-time.After(100 * time.Millisecond):
	}
	releaseFirstOnce.Do(func() { close(releaseFirst) })
	select {
	case <-lastReady:
	case <-time.After(5 * time.Second):
		t.Fatal("queued network did not start after worker slots were released")
	}
	select {
	case <-done:
		t.Fatal("cleanup did not wait for the final network operation")
	case <-time.After(100 * time.Millisecond):
	}
	releaseLastOnce.Do(func() { close(releaseLast) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish")
	}
	mu.Lock()
	defer mu.Unlock()
	if peak != 20 || completed != networkCount || active != 0 || len(seen) != networkCount {
		t.Errorf("peak=%d completed=%d active=%d unique=%d; want 20, %d, 0, %d", peak, completed, active, len(seen), networkCount, networkCount)
	}
}

func TestIsCustomStaticRoute(t *testing.T) {
	defaultRoute := &computepb.Route{
		Name: proto.String("default-route-123"), RouteType: proto.String("STATIC"),
		DestRange: proto.String("0.0.0.0/0"), Priority: proto.Uint32(1000),
		NextHopGateway: proto.String("https://www.googleapis.com/compute/v1/projects/test/global/gateways/default-internet-gateway"),
	}
	for _, tt := range []struct {
		name   string
		modify func(*computepb.Route)
		want   bool
	}{
		{name: "generated IPv4 default"},
		{name: "generated IPv6 default", modify: func(r *computepb.Route) { r.DestRange = proto.String("::/0") }},
		{name: "generated default without type", modify: func(r *computepb.Route) { r.RouteType = nil }},
		{name: "custom internet default", modify: func(r *computepb.Route) { r.Name = proto.String("custom-default") }, want: true},
		{name: "custom destination", modify: func(r *computepb.Route) { r.DestRange = proto.String("10.0.0.0/8") }, want: true},
		{name: "custom priority", modify: func(r *computepb.Route) { r.Priority = proto.Uint32(100) }, want: true},
		{name: "custom tags", modify: func(r *computepb.Route) { r.Tags = []string{"tag"} }, want: true},
		{name: "custom VPN default", modify: func(r *computepb.Route) {
			r.NextHopGateway = nil
			r.NextHopVpnTunnel = proto.String("vpn-tunnel")
		}, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := proto.Clone(defaultRoute).(*computepb.Route)
			if tt.modify != nil {
				tt.modify(route)
			}
			if got := isCustomStaticRoute(route); got != tt.want {
				t.Errorf("isCustomStaticRoute = %v, want %v", got, tt.want)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		route *computepb.Route
		want  bool
	}{
		{"static", &computepb.Route{RouteType: proto.String("STATIC")}, true},
		{"subnet", &computepb.Route{RouteType: proto.String("SUBNET")}, false},
		{"dynamic", &computepb.Route{RouteType: proto.String("BGP"), NextHopIp: proto.String("10.0.0.1")}, false},
		{"transit", &computepb.Route{RouteType: proto.String("TRANSIT")}, false},
		{"peering static", &computepb.Route{RouteType: proto.String("STATIC"), NextHopPeering: proto.String("peer")}, false},
		{"legacy subnet", &computepb.Route{NextHopNetwork: proto.String("network")}, false},
		{"legacy peering", &computepb.Route{NextHopPeering: proto.String("peer")}, false},
		{"legacy hub", &computepb.Route{NextHopHub: proto.String("hub")}, false},
		{"legacy IP", &computepb.Route{NextHopIp: proto.String("10.0.0.1")}, true},
		{"legacy instance", &computepb.Route{NextHopInstance: proto.String("instance")}, true},
		{"legacy gateway", &computepb.Route{NextHopGateway: proto.String("gateway")}, true},
		{"legacy VPN", &computepb.Route{NextHopVpnTunnel: proto.String("vpn")}, true},
		{"legacy ILB", &computepb.Route{NextHopIlb: proto.String("ilb")}, true},
		{"unknown", &computepb.Route{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCustomStaticRoute(tt.route); got != tt.want {
				t.Errorf("isCustomStaticRoute = %v, want %v", got, tt.want)
			}
		})
	}
}
