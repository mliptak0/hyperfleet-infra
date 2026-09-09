package sweep

import (
	"testing"
	"time"
)

func TestEvaluateResource(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	const runWindow = 24 * time.Hour

	tests := []struct {
		name           string
		resource       Resource
		expectedAction ActionType
	}{
		{
			name: "skip: cluster created within run window",
			resource: Resource{
				Name:        "hyperfleet-ci-e2e-1",
				Type:        ResourceCluster,
				TimeCreated: now.Add(-1 * time.Hour),
			},
			expectedAction: ActionSkip,
		},
		{
			name: "skip: resource created exactly at run window boundary",
			resource: Resource{
				Name:        "hyperfleet-ci-lb-1",
				Type:        ResourceLoadBalancer,
				TimeCreated: now.Add(-runWindow),
			},
			expectedAction: ActionSkip,
		},
		{
			name: "delete: cluster older than run window",
			resource: Resource{
				Name:        "hyperfleet-ci-e2e-2",
				Type:        ResourceCluster,
				TimeCreated: now.Add(-25 * time.Hour),
			},
			expectedAction: ActionDelete,
		},
		{
			name: "delete: block volume older than run window",
			resource: Resource{
				Name:        "hyperfleet-ci-vol-1",
				Type:        ResourceBlockVolume,
				TimeCreated: now.Add(-48 * time.Hour),
			},
			expectedAction: ActionDelete,
		},
		{
			name: "delete: db system older than run window",
			resource: Resource{
				Name:        "hyperfleet-ci-db-1",
				Type:        ResourceDBSystem,
				TimeCreated: now.Add(-72 * time.Hour),
			},
			expectedAction: ActionDelete,
		},
		{
			name: "skip: held by keep tag despite being old",
			resource: Resource{
				Name:         "hyperfleet-ci-e2e-3",
				Type:         ResourceCluster,
				TimeCreated:  now.Add(-72 * time.Hour),
				FreeformTags: map[string]string{TagKeep: "true"},
			},
			expectedAction: ActionSkip,
		},
		{
			name: "delete: keep tag set to non-true value does not exempt",
			resource: Resource{
				Name:         "hyperfleet-ci-e2e-4",
				Type:         ResourceCluster,
				TimeCreated:  now.Add(-72 * time.Hour),
				FreeformTags: map[string]string{TagKeep: "false"},
			},
			expectedAction: ActionDelete,
		},
		{
			name: "skip: zero-value creation timestamp is never deleted",
			resource: Resource{
				Name: "hyperfleet-ci-e2e-5",
				Type: ResourceCluster,
				// TimeCreated left at its zero value, as if the API omitted it.
			},
			expectedAction: ActionSkip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := EvaluateResource(tt.resource, now, runWindow)
			if decision.Action != tt.expectedAction {
				t.Errorf("EvaluateResource() action = %v, want %v (reason: %s)", decision.Action, tt.expectedAction, decision.Reason)
			}
		})
	}
}

func TestResourceTypeString(t *testing.T) {
	tests := []struct {
		rt   ResourceType
		want string
	}{
		{ResourceCluster, "cluster"},
		{ResourceLoadBalancer, "load-balancer"},
		{ResourceBlockVolume, "block-volume"},
		{ResourceDBSystem, "db-system"},
		{ResourceType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.rt.String(); got != tt.want {
			t.Errorf("ResourceType(%d).String() = %q, want %q", tt.rt, got, tt.want)
		}
	}
}
