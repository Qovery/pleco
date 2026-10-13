package gcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/container/apiv1/containerpb"
	"cloud.google.com/go/run/apiv2/runpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestVolatileObjectTTL(t *testing.T) {
	description := func(labels map[string]string) string {
		data, err := json.Marshal(labels)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	factories := []struct {
		name           string
		nativeCreation bool
		allowEpoch     bool
		makeObject     func(map[string]string, time.Time) VolatileObject
	}{
		{"bucket", true, false, func(labels map[string]string, created time.Time) VolatileObject {
			return Bucket{BucketAttrs: &storage.BucketAttrs{Name: "bucket", Labels: labels, Created: created}}
		}},
		{"repository", true, false, func(labels map[string]string, created time.Time) VolatileObject {
			return ArtifactRegistryRepository{Repository: &artifactregistrypb.Repository{Name: "repository", Labels: labels, CreateTime: timestamppb.New(created)}}
		}},
		{"cluster", false, false, func(labels map[string]string, _ time.Time) VolatileObject {
			return GKECluster{Cluster: &containerpb.Cluster{Name: "cluster", ResourceLabels: labels}}
		}},
		{"job", false, true, func(labels map[string]string, _ time.Time) VolatileObject {
			return Job{Job: &runpb.Job{Name: "job", Labels: labels}}
		}},
		{"network", false, false, func(labels map[string]string, _ time.Time) VolatileObject {
			return Network{Network: &computepb.Network{Name: proto.String("network"), Description: proto.String(description(labels))}}
		}},
		{"router", false, false, func(labels map[string]string, _ time.Time) VolatileObject {
			return Router{Router: &computepb.Router{Name: proto.String("router"), Description: proto.String(description(labels))}}
		}},
		{"service account", false, false, func(labels map[string]string, _ time.Time) VolatileObject {
			return ServiceAccount{ServiceAccount: &iam.ServiceAccount{Name: "account", Description: description(labels)}}
		}},
	}
	for _, factory := range factories {
		t.Run(factory.name, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				ttl     string
				created time.Time
				want    bool
			}{
				{"expired", "3600", time.Unix(1, 0), true},
				{"not expired", "3600", time.Now(), false},
				{"future creation", "3600", time.Now().Add(time.Hour), false},
				{"missing TTL", "", time.Unix(1, 0), false},
				{"blank TTL", " ", time.Unix(1, 0), false},
				{"invalid TTL", "invalid", time.Unix(1, 0), false},
				{"fractional TTL", "1.5", time.Unix(1, 0), false},
				{"overflowing TTL", "9223372036854775808", time.Unix(1, 0), false},
				{"protected", "0", time.Unix(1, 0), false},
				{"legacy negative TTL", "-1", time.Unix(1, 0), true},
			} {
				t.Run(tt.name, func(t *testing.T) {
					labels := map[string]string{"creation_date": strconv.FormatInt(tt.created.Unix(), 10)}
					if tt.ttl != "" {
						labels["ttl"] = tt.ttl
					}
					if got := factory.makeObject(labels, tt.created).IsExpired(); got != tt.want {
						t.Errorf("IsExpired() = %v, want %v", got, tt.want)
					}
				})
			}
			for _, creation := range []string{"", "invalid", "1.5", "9223372036854775808", "0", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)} {
				t.Run("creation_date="+creation, func(t *testing.T) {
					labels := map[string]string{"ttl": "3600"}
					if creation != "" {
						labels["creation_date"] = creation
					}
					// Buckets and repositories use their native creation timestamp;
					// jobs alone accept an explicit zero-valued creation_date.
					want := factory.nativeCreation || (factory.allowEpoch && creation == "0")
					if got := factory.makeObject(labels, time.Unix(1, 0)).IsExpired(); got != want {
						t.Errorf("IsExpired() = %v, want %v", got, want)
					}
				})
			}
			if factory.nativeCreation {
				t.Run("old label cannot override recent native creation", func(t *testing.T) {
					if factory.makeObject(map[string]string{"ttl": "3600", "creation_date": "1"}, time.Now()).IsExpired() {
						t.Error("expired based on label instead of native creation time")
					}
				})
			}
		})
	}
}

func TestVolatileObjectDescription(t *testing.T) {
	for _, tt := range []struct {
		description string
		want        bool
	}{
		{`{"ttl":3600,"creation_date":1}`, true},
		{`{"ttl":"3600","creation_date":"1"}`, true},
		{`{"ttl":3600}`, false},
		{`{"creation_date":1}`, false},
		{`{"ttl":null,"creation_date":1}`, false},
		{`{"ttl":3600,"creation_date":null}`, false},
		{`{"ttl":true,"creation_date":1}`, false},
		{`{"ttl":3600,"creation_date":{}}`, false},
		{"", false},
		{"not JSON", false},
		{"null", false},
		{"[]", false},
	} {
		t.Run(tt.description, func(t *testing.T) {
			for _, object := range []VolatileObject{
				Network{Network: &computepb.Network{Description: proto.String(tt.description)}},
				Router{Router: &computepb.Router{Description: proto.String(tt.description)}},
				ServiceAccount{ServiceAccount: &iam.ServiceAccount{Description: tt.description}},
			} {
				if got := object.IsExpired(); got != tt.want {
					t.Errorf("%T.IsExpired() = %v, want %v", object, got, tt.want)
				}
			}
		})
	}
}

func TestVolatileObjectZeroValues(t *testing.T) {
	for _, object := range []VolatileObject{
		Bucket{}, ArtifactRegistryRepository{}, GKECluster{}, Job{}, Network{},
		Router{}, ServiceAccount{}, Disk{}, DeletedServiceAccountBinding{}, MissingServiceAccountBinding{},
	} {
		if object.IsExpired() {
			t.Errorf("zero-valued %T is expired", object)
		}
	}
}

func TestDiskIsExpiredRequiresMissingOwner(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"existing or unknown owner", nil, false},
		{"REST not found", &googleapi.Error{Code: http.StatusNotFound}, true},
		{"gRPC not found", status.Error(codes.NotFound, "missing cluster"), true},
		{"wrapped not found", fmt.Errorf("lookup: %w", &googleapi.Error{Code: http.StatusNotFound}), true},
		{"permission denied", &googleapi.Error{Code: http.StatusForbidden}, false},
		{"unavailable", status.Error(codes.Unavailable, "unavailable"), false},
		{"unrecognized error", errors.New("lookup failed"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var object VolatileObject = Disk{Disk: orphanDisk(), OwnerLookupError: tt.err}
			if got := object.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIAMBindingsIsExpired(t *testing.T) {
	live := "serviceAccount:live@" + testProjectID + ".iam.gserviceaccount.com"
	missing := "serviceAccount:missing@" + testProjectID + ".iam.gserviceaccount.com"
	for _, tt := range []struct {
		member      string
		wantDeleted bool
		wantMissing bool
	}{
		{live, false, false},
		{missing, false, true},
		{"deleted:" + missing + "?uid=123", true, false},
		{"serviceAccount:account@another-project.iam.gserviceaccount.com", false, false},
		{"serviceAccount:service-123@container-engine-robot.iam.gserviceaccount.com", false, false},
		{"user:user@example.com", false, false},
		{"deleted:user:user@example.com?uid=123", false, false},
		{"", false, false},
	} {
		t.Run(tt.member, func(t *testing.T) {
			var deleted VolatileObject = DeletedServiceAccountBinding{Member: tt.member}
			if got := deleted.IsExpired(); got != tt.wantDeleted {
				t.Errorf("deleted binding IsExpired() = %v, want %v", got, tt.wantDeleted)
			}
			var absent VolatileObject = MissingServiceAccountBinding{
				Member: tt.member, ProjectID: testProjectID, ExistingServiceAccounts: map[string]struct{}{live: {}},
			}
			if got := absent.IsExpired(); got != tt.wantMissing {
				t.Errorf("missing account binding IsExpired() = %v, want %v", got, tt.wantMissing)
			}
		})
	}
	if (MissingServiceAccountBinding{Member: missing, ProjectID: testProjectID}).IsExpired() {
		t.Error("an unknown inventory must not prove an account is missing")
	}
	if !(MissingServiceAccountBinding{Member: missing, ProjectID: testProjectID, ExistingServiceAccounts: map[string]struct{}{}}).IsExpired() {
		t.Error("a complete empty inventory must prove a project account is missing")
	}
}
