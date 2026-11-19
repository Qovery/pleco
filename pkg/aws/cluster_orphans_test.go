package aws

import (
	"testing"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

func TestIsOrphanOfDeletedCluster(t *testing.T) {
	now := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour)
	liveClusters := []string{"qovery-z0a4996dd", "byok-v2-test"}

	tests := []struct {
		name      string
		resource  common.CloudProviderResource
		clusterId string
		want      bool
	}{
		{"cluster gone, no ttl tag", common.CloudProviderResource{CreationDate: old, TTL: -1}, "z3f5caf5b", true},
		{"cluster gone, ttl 0", common.CloudProviderResource{CreationDate: old, TTL: 0}, "z7fc7e4a2", true},
		{"cluster gone, ttl not expired", common.CloudProviderResource{CreationDate: old, TTL: 999999}, "z3f5caf5b", true},
		{"cluster still exists", common.CloudProviderResource{CreationDate: old, TTL: -1}, "z0a4996dd", false},
		{"no ClusterId tag", common.CloudProviderResource{CreationDate: old, TTL: -1}, "", false},
		{"protected by do_not_delete", common.CloudProviderResource{CreationDate: old, TTL: -1, IsProtected: true}, "z3f5caf5b", false},
		{"created within the grace period", common.CloudProviderResource{CreationDate: now.Add(-23 * time.Hour), TTL: -1}, "z3f5caf5b", false},
		{"no creationDate tag", common.CloudProviderResource{TTL: -1}, "z3f5caf5b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOrphanOfDeletedCluster(tt.resource, tt.clusterId, liveClusters, now); got != tt.want {
				t.Errorf("isOrphanOfDeletedCluster() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestIsOrphanOfDeletedClusterWithNoLiveCluster(t *testing.T) {
	now := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
	resource := common.CloudProviderResource{CreationDate: now.Add(-48 * time.Hour), TTL: -1}

	if !isOrphanOfDeletedCluster(resource, "z3f5caf5b", nil, now) {
		t.Error("a resource of a cluster that is gone must be orphaned when no cluster is left")
	}
}

func TestQoveryClusterResourceNames(t *testing.T) {
	if !isQoveryClusterEventRule("qovery-cw-event-SpotInterrupt-20250717223224132000000003") {
		t.Error("Qovery cluster event rule not recognized")
	}
	if isQoveryClusterEventRule("AutoScalingManagedRule") {
		t.Error("AWS-managed rule recognized as a Qovery cluster rule")
	}

	if !isQoveryClusterQueue("https://sqs.eu-west-3.amazonaws.com/843237546537/qovery-z3f5caf5b", "z3f5caf5b") {
		t.Error("Qovery cluster queue not recognized")
	}
	if isQoveryClusterQueue("https://sqs.eu-west-3.amazonaws.com/843237546537/customer-queue", "z3f5caf5b") {
		t.Error("queue with another name recognized as the cluster queue")
	}

	for _, name := range []string{"vpc-0f1658cf3ab34a6d5", "documentdb-vpc-0f1658cf3ab34a6d5"} {
		if !isQoveryClusterSubnetGroup(name) {
			t.Errorf("Qovery cluster subnet group %s not recognized", name)
		}
	}
	if isQoveryClusterSubnetGroup("default") {
		t.Error("default subnet group recognized as a Qovery cluster subnet group")
	}
}
