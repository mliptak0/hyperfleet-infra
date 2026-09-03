# OCI CI Compartment Sweep

An OCI Function (Go) that sweeps the `hyperfleet-ci` compartment on a schedule,
deleting anything older than the run window: OKE clusters, load balancers,
block volumes, and DB systems. It is the backstop for
[HYPERFLEET-1563](https://redhat.atlassian.net/browse/HYPERFLEET-1563)'s
per-run teardown — resources that survive a failed or killed CI run still get
cleaned up here.

Deployed and scheduled via Terraform: [`terraform/modules/lifecycle/oci/`](../../terraform/modules/lifecycle/oci/).

## How it works

1. OCI Resource Scheduler invokes the function on a cron schedule (default
   hourly).
2. The function authenticates as a resource principal (no embedded
   credentials) and lists clusters, load balancers, block volumes, and DB
   systems in `COMPARTMENT_ID`.
3. `EvaluateResource()` (in `internal/sweep`) decides, per resource, whether
   its age exceeds `RUN_WINDOW_HOURS`. A resource tagged
   `hyperfleet-keep=true` is held regardless of age, for manual debugging.
4. Resources marked for deletion are deleted, unless `DRY_RUN=true`, in which
   case the action is only logged.
5. A JSON summary (per-resource action, reason, and outcome) is returned and
   logged — this is what satisfies the "sweep runs on a schedule and its log
   shows what it removed" acceptance criterion.

The decision logic (`internal/sweep`) has no OCI SDK dependency and is
independently unit-tested; `main.go` wires it to the OCI SDK and the
[`fdk-go`](https://github.com/fnproject/fdk-go) Functions runtime.

### Scope

The sweep lists and deletes exactly four resource types: OKE clusters,
(classic) load balancers, block volumes, and DB systems. It deliberately
does **not** touch:

- Network load balancers (the newer NLB service — a different API from the
  classic load balancers it does sweep)
- Standalone compute instances and node pools
- VCNs, subnets, and other network resources
- Object storage buckets

Anything a CI run creates outside those four types has to be cleaned up by
the run itself; the compartment quota and budget are the backstops for the
rest. A delete that comes back as a 409/`IncorrectState` (the resource is
already terminating, typically from the CI job's own teardown) is recorded
as a skip, not a failure — the next run re-evaluates it.

## Configuration (function config / environment variables)

| Variable           | Default    | Description                                      |
| ------------------ | ---------- | ------------------------------------------------- |
| `COMPARTMENT_ID`   | *(required)* | OCID of the compartment to sweep                |
| `RUN_WINDOW_HOURS`  | `8`        | Age past which a resource is swept                |
| `DRY_RUN`           | `true`     | Set to `false` to actually delete resources       |

## Development

```bash
make test-oci-sweep-function
make build-oci-sweep-function
make lint-oci-sweep-function
```

## Deployment

Build and push the image by tag, then pin Terraform to its immutable
@sha256 digest. For the rhelcert tenancy in us-sanjose-1, that's region key
`sjc` and namespace `axpiwif30tzw` (confirmed 2026-09-02). OCI Functions
requires a **tag** reference in `sweep_function_image` and rejects a digest
reference there ("Image is not a valid docker image name"); the immutable pin
goes in `sweep_function_image_digest`, which is what OCI actually pulls by.
Repository immutability isn't supported by the Artifacts API in us-sanjose-1
(see `terraform/modules/lifecycle/oci/functions.tf`), so that digest — not the
tag — is what guards against the deployed content being silently swapped, and
Terraform validation enforces the split (tag in one variable, `sha256:` digest
in the other):

```bash
cd functions/oci-ci-sweep
TAG=$(git rev-parse --short HEAD)
REPO=sjc.ocir.io/axpiwif30tzw/oci-ci-sweep
docker build --platform linux/amd64 -t "$REPO:$TAG" .
docker push "$REPO:$TAG"

# Read back the digest the push produced:
docker inspect --format='{{index .RepoDigests 0}}' "$REPO:$TAG"
# e.g. sjc.ocir.io/axpiwif30tzw/oci-ci-sweep@sha256:<64-hex>
# In ci.tfvars set:
#   sweep_function_image        = "sjc.ocir.io/axpiwif30tzw/oci-ci-sweep:<TAG>"
#   sweep_function_image_digest = "sha256:<64-hex>"   # the part after the @
```

Terraform creates the OCIR repository (see the
`sweep_container_repository_path` output of `terraform/oci/`) but does not
build or push the image — that stays a CI/manual step, same division of
labor as the rest of this repo's container images.

### Code structure

| File                        | Purpose                                                        |
| --------------------------- | ---------------------------------------------------------------- |
| `internal/sweep/decision.go` | Pure sweep decision logic — no OCI SDK dependency, unit-testable |
| `internal/sweep/decision_test.go` | Table-driven tests covering all decision scenarios          |
| `main.go`                   | Function entry point, OCI SDK clients, action executor           |
