package sweep

import "time"

const (
	// TagKeep, when set to "true" as a freeform tag, exempts a resource from
	// the sweep regardless of age. Used to hold a resource during manual debugging.
	TagKeep = "hyperfleet-keep"
)

type ResourceType int

const (
	ResourceCluster ResourceType = iota
	ResourceLoadBalancer
	ResourceBlockVolume
	ResourceDBSystem
)

func (t ResourceType) String() string {
	switch t {
	case ResourceCluster:
		return "cluster"
	case ResourceLoadBalancer:
		return "load-balancer"
	case ResourceBlockVolume:
		return "block-volume"
	case ResourceDBSystem:
		return "db-system"
	default:
		return "unknown"
	}
}

// Resource describes an OCI resource found in the CI compartment, in
// cloud-agnostic terms so EvaluateResource has no OCI SDK dependency.
type Resource struct {
	OCID         string
	Name         string
	Type         ResourceType
	TimeCreated  time.Time
	FreeformTags map[string]string
}

type ActionType int

const (
	ActionSkip ActionType = iota
	ActionDelete
)

func (a ActionType) String() string {
	switch a {
	case ActionSkip:
		return "skip"
	case ActionDelete:
		return "delete"
	default:
		return "unknown"
	}
}

type Decision struct {
	Action ActionType
	Reason string
}

// EvaluateResource decides whether a resource should be deleted by the sweep.
// A resource is deleted once it is older than runWindow, unless it carries the
// TagKeep freeform tag set to "true".
func EvaluateResource(r Resource, now time.Time, runWindow time.Duration) Decision {
	if r.FreeformTags[TagKeep] == "true" {
		return Decision{Action: ActionSkip, Reason: "held by " + TagKeep + " tag"}
	}

	if r.TimeCreated.IsZero() {
		return Decision{Action: ActionSkip, Reason: "no creation timestamp available"}
	}

	age := now.Sub(r.TimeCreated)
	if age <= runWindow {
		return Decision{Action: ActionSkip, Reason: "within run window"}
	}

	return Decision{Action: ActionDelete, Reason: "older than run window (age=" + age.Round(time.Minute).String() + ")"}
}
