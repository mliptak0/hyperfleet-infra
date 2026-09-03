package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/fdk-go"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"

	"github.com/openshift-hyperfleet/hyperfleet-infra/functions/oci-ci-sweep/internal/sweep"
)

const (
	defaultRunWindowHours = 8
	// maxRunWindowHours bounds RUN_WINDOW_HOURS so a misconfiguration (a
	// negative value, or one large enough to overflow the Duration
	// conversion) can't silently turn into "delete everything".
	maxRunWindowHours = 8760 // 1 year
)

func main() {
	fdk.Handle(fdk.HandlerFunc(handleSweep))
}

type sweepResult struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	OCID     string `json:"ocid"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	Executed bool   `json:"executed"`
	Error    string `json:"error,omitempty"`
}

type ociClients struct {
	containerEngine containerengine.ContainerEngineClient
	loadBalancer    loadbalancer.LoadBalancerClient
	blockStorage    core.BlockstorageClient
	database        database.DatabaseClient
}

func handleSweep(ctx context.Context, in io.Reader, out io.Writer) {
	logger := slog.Default()
	now := time.Now().UTC()

	compartmentID := os.Getenv("COMPARTMENT_ID")
	if compartmentID == "" {
		logger.Error("COMPARTMENT_ID environment variable is required")
		writeError(out, "COMPARTMENT_ID environment variable is required")
		return
	}

	runWindow := defaultRunWindowHours * time.Hour
	if v := os.Getenv("RUN_WINDOW_HOURS"); v != "" {
		hours, err := strconv.Atoi(v)
		if err != nil {
			logger.Error("invalid RUN_WINDOW_HOURS", "value", v, "error", err)
			writeError(out, "invalid RUN_WINDOW_HOURS: "+err.Error())
			return
		}
		if hours <= 0 || hours > maxRunWindowHours {
			msg := fmt.Sprintf("RUN_WINDOW_HOURS must be between 1 and %d, got %d", maxRunWindowHours, hours)
			logger.Error(msg)
			writeError(out, msg)
			return
		}
		runWindow = time.Duration(hours) * time.Hour
	}

	dryRun := os.Getenv("DRY_RUN") != "false"

	logger.Info("starting CI compartment sweep",
		"compartment", compartmentID,
		"run_window", runWindow.String(),
		"dry_run", dryRun,
		"timestamp", now.Format(time.RFC3339),
	)

	provider, err := auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		logger.Error("failed to create resource principal provider", "error", err)
		writeError(out, "failed to create resource principal provider")
		return
	}

	clients, err := initOCIClients(provider)
	if err != nil {
		logger.Error("failed to initialize OCI clients", "error", err)
		writeError(out, "failed to initialize OCI clients: "+err.Error())
		return
	}

	resources, listErrs := listAllResources(ctx, provider, compartmentID)
	for _, e := range listErrs {
		logger.Error("failed to list resources", "error", e)
	}

	logger.Info("found resources", "count", len(resources))

	results := make([]sweepResult, 0, len(resources))
	hadFailure := len(listErrs) > 0

	for _, r := range resources {
		decision := sweep.EvaluateResource(r, now, runWindow)

		logger.Info("evaluated resource",
			"type", r.Type.String(),
			"name", r.Name,
			"ocid", r.OCID,
			"action", decision.Action.String(),
			"reason", decision.Reason,
		)

		res := sweepResult{
			Type:   r.Type.String(),
			Name:   r.Name,
			OCID:   r.OCID,
			Action: decision.Action.String(),
			Reason: decision.Reason,
		}

		if decision.Action != sweep.ActionDelete {
			results = append(results, res)
			continue
		}

		if dryRun {
			logger.Info("DRY RUN: would delete resource", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID)
			results = append(results, res)
			continue
		}

		// Refetch to guard against TOCTOU: re-evaluate current state before deleting
		refetched, err := refetchResource(ctx, clients, r)
		if err != nil {
			hadFailure = true
			res.Error = fmt.Sprintf("refetch failed: %v", err)
			logger.Error("failed to refetch resource before deletion", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID, "error", err)
			results = append(results, res)
			continue
		}
		if refetched == nil {
			// Resource was deleted by someone else in the meantime.
			res.Executed = true
			logger.Info("resource already deleted", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID)
			results = append(results, res)
			continue
		}

		// Re-evaluate: if it no longer qualifies for deletion, skip it.
		updatedDecision := sweep.EvaluateResource(*refetched, now, runWindow)
		if updatedDecision.Action != sweep.ActionDelete {
			logger.Info("resource no longer qualifies for deletion after refetch", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID, "reason", updatedDecision.Reason)
			res.Action = updatedDecision.Action.String()
			res.Reason = updatedDecision.Reason
			results = append(results, res)
			continue
		}

		if err := deleteResource(ctx, clients, *refetched); err != nil {
			if isConflictError(err) {
				// The resource is mid-transition (typically already
				// terminating from the CI job's own teardown, or otherwise in
				// a state that blocks deletion right now). This isn't a
				// failure: the next scheduled run re-evaluates it and finds it
				// either gone or ready to delete. Record it as a skip so it
				// doesn't fail the whole run.
				res.Action = sweep.ActionSkip.String()
				res.Reason = fmt.Sprintf("deletion deferred (resource busy or already terminating): %v", err)
				logger.Info("deletion deferred; will retry next run", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID, "error", err)
			} else {
				hadFailure = true
				res.Error = err.Error()
				logger.Error("failed to delete resource", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID, "error", err)
			}
		} else {
			res.Executed = true
			logger.Info("deleted resource", "type", r.Type.String(), "name", r.Name, "ocid", r.OCID)
		}
		results = append(results, res)
	}

	response := map[string]any{
		"timestamp":   now.Format(time.RFC3339),
		"compartment": compartmentID,
		"dry_run":     dryRun,
		"results":     results,
	}
	if hadFailure {
		fdk.WriteStatus(out, 500)
	}
	if err := json.NewEncoder(out).Encode(response); err != nil {
		logger.Error("failed to encode response", "error", err)
	}
}

func writeError(out io.Writer, msg string) {
	logger := slog.Default()
	fdk.WriteStatus(out, 500)
	if err := json.NewEncoder(out).Encode(map[string]string{"error": msg}); err != nil {
		logger.Error("failed to encode error response", "error", err)
	}
}

func listAllResources(ctx context.Context, provider common.ConfigurationProvider, compartmentID string) ([]sweep.Resource, []error) {
	var resources []sweep.Resource
	var errs []error

	if r, err := listClusters(ctx, provider, compartmentID); err != nil {
		errs = append(errs, fmt.Errorf("listing clusters: %w", err))
	} else {
		resources = append(resources, r...)
	}

	if r, err := listLoadBalancers(ctx, provider, compartmentID); err != nil {
		errs = append(errs, fmt.Errorf("listing load balancers: %w", err))
	} else {
		resources = append(resources, r...)
	}

	if r, err := listBlockVolumes(ctx, provider, compartmentID); err != nil {
		errs = append(errs, fmt.Errorf("listing block volumes: %w", err))
	} else {
		resources = append(resources, r...)
	}

	if r, err := listDBSystems(ctx, provider, compartmentID); err != nil {
		errs = append(errs, fmt.Errorf("listing db systems: %w", err))
	} else {
		resources = append(resources, r...)
	}

	return resources, errs
}

func listClusters(ctx context.Context, provider common.ConfigurationProvider, compartmentID string) ([]sweep.Resource, error) {
	client, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating container engine client: %w", err)
	}

	var resources []sweep.Resource
	var page *string
	for {
		resp, err := client.ListClusters(ctx, containerengine.ListClustersRequest{
			CompartmentId: &compartmentID,
			LifecycleState: []containerengine.ClusterLifecycleStateEnum{
				containerengine.ClusterLifecycleStateActive,
				containerengine.ClusterLifecycleStateCreating,
				containerengine.ClusterLifecycleStateFailed,
			},
			Page: page,
		})
		if err != nil {
			return nil, err
		}

		for _, c := range resp.Items {
			res := sweep.Resource{
				OCID:         valueOrEmpty(c.Id),
				Name:         valueOrEmpty(c.Name),
				Type:         sweep.ResourceCluster,
				FreeformTags: c.FreeformTags,
			}
			if c.Metadata != nil && c.Metadata.TimeCreated != nil {
				res.TimeCreated = c.Metadata.TimeCreated.Time
			}
			resources = append(resources, res)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return resources, nil
}

func listLoadBalancers(ctx context.Context, provider common.ConfigurationProvider, compartmentID string) ([]sweep.Resource, error) {
	client, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating load balancer client: %w", err)
	}

	var resources []sweep.Resource
	var page *string
	for {
		resp, err := client.ListLoadBalancers(ctx, loadbalancer.ListLoadBalancersRequest{
			CompartmentId: &compartmentID,
			Page:          page,
		})
		if err != nil {
			return nil, err
		}

		for _, lb := range resp.Items {
			if lb.LifecycleState == loadbalancer.LoadBalancerLifecycleStateDeleting ||
				lb.LifecycleState == loadbalancer.LoadBalancerLifecycleStateDeleted {
				continue
			}
			res := sweep.Resource{
				OCID:         valueOrEmpty(lb.Id),
				Name:         valueOrEmpty(lb.DisplayName),
				Type:         sweep.ResourceLoadBalancer,
				FreeformTags: lb.FreeformTags,
			}
			if lb.TimeCreated != nil {
				res.TimeCreated = lb.TimeCreated.Time
			}
			resources = append(resources, res)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return resources, nil
}

func listBlockVolumes(ctx context.Context, provider common.ConfigurationProvider, compartmentID string) ([]sweep.Resource, error) {
	client, err := core.NewBlockstorageClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating blockstorage client: %w", err)
	}

	var resources []sweep.Resource
	var page *string
	for {
		resp, err := client.ListVolumes(ctx, core.ListVolumesRequest{
			CompartmentId: &compartmentID,
			Page:          page,
		})
		if err != nil {
			return nil, err
		}

		for _, v := range resp.Items {
			if v.LifecycleState == core.VolumeLifecycleStateTerminating ||
				v.LifecycleState == core.VolumeLifecycleStateTerminated {
				continue
			}
			res := sweep.Resource{
				OCID:         valueOrEmpty(v.Id),
				Name:         valueOrEmpty(v.DisplayName),
				Type:         sweep.ResourceBlockVolume,
				FreeformTags: v.FreeformTags,
			}
			if v.TimeCreated != nil {
				res.TimeCreated = v.TimeCreated.Time
			}
			resources = append(resources, res)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return resources, nil
}

func listDBSystems(ctx context.Context, provider common.ConfigurationProvider, compartmentID string) ([]sweep.Resource, error) {
	client, err := database.NewDatabaseClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating database client: %w", err)
	}

	var resources []sweep.Resource
	var page *string
	for {
		resp, err := client.ListDbSystems(ctx, database.ListDbSystemsRequest{
			CompartmentId: &compartmentID,
			Page:          page,
		})
		if err != nil {
			return nil, err
		}

		for _, db := range resp.Items {
			if db.LifecycleState == database.DbSystemSummaryLifecycleStateTerminating ||
				db.LifecycleState == database.DbSystemSummaryLifecycleStateTerminated {
				continue
			}
			res := sweep.Resource{
				OCID:         valueOrEmpty(db.Id),
				Name:         valueOrEmpty(db.DisplayName),
				Type:         sweep.ResourceDBSystem,
				FreeformTags: db.FreeformTags,
			}
			if db.TimeCreated != nil {
				res.TimeCreated = db.TimeCreated.Time
			}
			resources = append(resources, res)
		}

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return resources, nil
}

func initOCIClients(provider common.ConfigurationProvider) (*ociClients, error) {
	ceClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating container engine client: %w", err)
	}
	lbClient, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating load balancer client: %w", err)
	}
	bsClient, err := core.NewBlockstorageClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating blockstore client: %w", err)
	}
	dbClient, err := database.NewDatabaseClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("creating database client: %w", err)
	}
	return &ociClients{
		containerEngine: ceClient,
		loadBalancer:    lbClient,
		blockStorage:    bsClient,
		database:        dbClient,
	}, nil
}

// isNotFoundError reports whether err is an OCI HTTP 404 response, meaning the
// resource no longer exists. Like isConflictError, it trusts a structured
// ServiceError's status code and returns immediately — otherwise a non-404
// error whose message, OPC request ID, or OCID happens to contain "404" would
// be misread as "already deleted", and refetchResource's caller would mark a
// still-existing resource as gone and lose the failure signal (CWE-754). Only
// when the error doesn't unwrap to a ServiceError does it fall back to string
// matching.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var svcErr common.ServiceError
	if errors.As(err, &svcErr) {
		return svcErr.GetHTTPStatusCode() == http.StatusNotFound
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "notfound")
}

// refetchResource checks if a resource still exists and retrieves current tags.
// Returns nil if the resource no longer exists (already deleted). It keeps the
// original TimeCreated: the age doesn't change between the list and this GET, so
// there's no need to re-read it (only the freeform tags, which gate the
// hyperfleet-keep exemption, can have changed).
func refetchResource(ctx context.Context, clients *ociClients, r sweep.Resource) (*sweep.Resource, error) {
	switch r.Type {
	case sweep.ResourceCluster:
		resp, err := clients.containerEngine.GetCluster(ctx, containerengine.GetClusterRequest{ClusterId: &r.OCID})
		if err != nil {
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("fetching cluster: %w", err)
		}
		result := r
		result.FreeformTags = resp.Cluster.FreeformTags
		return &result, nil

	case sweep.ResourceLoadBalancer:
		resp, err := clients.loadBalancer.GetLoadBalancer(ctx, loadbalancer.GetLoadBalancerRequest{LoadBalancerId: &r.OCID})
		if err != nil {
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("fetching load balancer: %w", err)
		}
		result := r
		result.FreeformTags = resp.LoadBalancer.FreeformTags
		return &result, nil

	case sweep.ResourceBlockVolume:
		resp, err := clients.blockStorage.GetVolume(ctx, core.GetVolumeRequest{VolumeId: &r.OCID})
		if err != nil {
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("fetching block volume: %w", err)
		}
		result := r
		result.FreeformTags = resp.Volume.FreeformTags
		return &result, nil

	case sweep.ResourceDBSystem:
		resp, err := clients.database.GetDbSystem(ctx, database.GetDbSystemRequest{DbSystemId: &r.OCID})
		if err != nil {
			if isNotFoundError(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("fetching db system: %w", err)
		}
		result := r
		result.FreeformTags = resp.DbSystem.FreeformTags
		return &result, nil

	default:
		return nil, fmt.Errorf("unsupported resource type %q", r.Type.String())
	}
}

// isConflictError reports whether err is an OCI 409/IncorrectState response,
// meaning the resource can't be deleted right now because it's mid-transition
// (e.g. already terminating). The sweep treats these as a skip, not a failure:
// the next scheduled run re-evaluates the resource. It checks the structured
// ServiceError first and falls back to string matching for errors that don't
// unwrap to one.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	// If it unwraps to a structured ServiceError, trust the status/code and
	// return immediately — do not fall through to string matching. Otherwise
	// an unrelated error (e.g. a 500) whose message, OPC request ID, or OCID
	// happens to contain "409" would be misread as a conflict and quietly
	// skipped, when that's exactly the case we want to fail the run so a human
	// looks at it.
	var svcErr common.ServiceError
	if errors.As(err, &svcErr) {
		return svcErr.GetHTTPStatusCode() == http.StatusConflict ||
			strings.EqualFold(svcErr.GetCode(), "IncorrectState")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "409") ||
		strings.Contains(msg, "incorrectstate") ||
		strings.Contains(msg, "already terminating")
}

func deleteResource(ctx context.Context, clients *ociClients, r sweep.Resource) error {
	switch r.Type {
	case sweep.ResourceCluster:
		_, err := clients.containerEngine.DeleteCluster(ctx, containerengine.DeleteClusterRequest{ClusterId: &r.OCID})
		if err != nil {
			return fmt.Errorf("deleting cluster: %w", err)
		}
		return nil

	case sweep.ResourceLoadBalancer:
		_, err := clients.loadBalancer.DeleteLoadBalancer(ctx, loadbalancer.DeleteLoadBalancerRequest{LoadBalancerId: &r.OCID})
		if err != nil {
			return fmt.Errorf("deleting load balancer: %w", err)
		}
		return nil

	case sweep.ResourceBlockVolume:
		_, err := clients.blockStorage.DeleteVolume(ctx, core.DeleteVolumeRequest{VolumeId: &r.OCID})
		if err != nil {
			return fmt.Errorf("deleting block volume: %w", err)
		}
		return nil

	case sweep.ResourceDBSystem:
		_, err := clients.database.TerminateDbSystem(ctx, database.TerminateDbSystemRequest{DbSystemId: &r.OCID})
		if err != nil {
			return fmt.Errorf("terminating db system: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported resource type %q", r.Type.String())
	}
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
