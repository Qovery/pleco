package gcp

import (
	"testing"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
)

const testProjectID = "qovery-gcp-tests"

func TestIsProjectOwnedServiceAccount(t *testing.T) {
	tests := []struct {
		name   string
		member string
		want   bool
	}{
		{
			name:   "service account owned by the project",
			member: "serviceAccount:qovery@qovery-gcp-tests.iam.gserviceaccount.com",
			want:   true,
		},
		{
			// Memorystore Redis uses the legacy @cloud-redis domain, absent from
			// gcpManagedSADomains. Stripping this binding makes every Memorystore
			// instance create fail with FAILED_PRECONDITION.
			name:   "memorystore redis service agent",
			member: "serviceAccount:service-809315353539@cloud-redis.iam.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "service networking service agent",
			member: "serviceAccount:service-809315353539@service-networking.iam.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "gke service agent",
			member: "serviceAccount:service-809315353539@container-engine-robot.iam.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "gcp-sa service agent",
			member: "serviceAccount:service-809315353539@gcp-sa-artifactregistry.iam.gserviceaccount.com",
			want:   false,
		},
		{
			// No service- prefix, so isGCPManagedMember never protected it either.
			name:   "cloud build service agent",
			member: "serviceAccount:809315353539@cloudbuild.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "service account owned by another project",
			member: "serviceAccount:ci@some-other-project.iam.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "project id appearing as a suffix of another project id",
			member: "serviceAccount:ci@not-qovery-gcp-tests.iam.gserviceaccount.com",
			want:   false,
		},
		{
			name:   "user member",
			member: "user:apromerova@qovery.com",
			want:   false,
		},
		{
			name:   "group member",
			member: "group:sre@qovery.com",
			want:   false,
		},
		{
			// Handled by DeleteOrphanedIAMPolicyBindings instead.
			name:   "tombstoned service account",
			member: "deleted:serviceAccount:gone@qovery-gcp-tests.iam.gserviceaccount.com",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isProjectOwnedServiceAccount(tt.member, testProjectID); got != tt.want {
				t.Errorf("isProjectOwnedServiceAccount(%q, %q) = %v, want %v",
					tt.member, testProjectID, got, tt.want)
			}
		})
	}
}

func TestFindNonExistentProjectServiceAccountsSkipsServiceAgents(t *testing.T) {
	policy := &cloudresourcemanager.Policy{
		Bindings: []*cloudresourcemanager.Binding{
			{
				Role:    "roles/redis.serviceAgent",
				Members: []string{"serviceAccount:service-809315353539@cloud-redis.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/servicenetworking.serviceAgent",
				Members: []string{"serviceAccount:service-809315353539@service-networking.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/container.serviceAgent",
				Members: []string{"serviceAccount:service-809315353539@container-engine-robot.iam.gserviceaccount.com"},
			},
			{
				Role: "roles/redis.admin",
				Members: []string{
					"serviceAccount:qovery@qovery-gcp-tests.iam.gserviceaccount.com",
					"serviceAccount:reaped@qovery-gcp-tests.iam.gserviceaccount.com",
					"serviceAccount:ci@some-other-project.iam.gserviceaccount.com",
					"user:apromerova@qovery.com",
				},
			},
		},
	}

	existingSAs := map[string]struct{}{
		"serviceAccount:qovery@qovery-gcp-tests.iam.gserviceaccount.com": {},
	}

	got := findNonExistentProjectServiceAccounts(policy, existingSAs, testProjectID)

	want := map[string]struct{}{
		"serviceAccount:reaped@qovery-gcp-tests.iam.gserviceaccount.com": {},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d members %v, want %d members %v", len(got), keys(got), len(want), keys(want))
	}
	for member := range want {
		if _, ok := got[member]; !ok {
			t.Errorf("expected %q to be reported as non-existent, got %v", member, keys(got))
		}
	}
}

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
