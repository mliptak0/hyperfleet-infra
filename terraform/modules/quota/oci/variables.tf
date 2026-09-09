variable "tenancy_ocid" {
  description = "Tenancy OCID. Quota policies must be created at the tenancy root compartment."
  type        = string
}

variable "name" {
  description = "Name of the quota policy."
  type        = string
  default     = "hyperfleet-ci-quota"
}

variable "description" {
  description = "Description of the quota policy."
  type        = string
  default     = "Caps compute cores and OKE clusters in the hyperfleet-ci compartment."
}

variable "statements" {
  description = <<-EOT
    Quota policy statements (OCI quota policy language), e.g.
    ["set compute-core quota standard-e4-core-count to 16 in compartment HyperFleet:hyperfleet-ci"].

    There is no hardcoded default here because the module is tenancy-agnostic:
    the exact quota-family and quota-name strings depend on the worker node
    shape chosen for the CI compartment's OKE clusters, and whether Container
    Engine exposes a compartment-quota-manageable cluster count for this
    tenancy. Discover the authoritative values before setting this:

      oci limits definition list --compartment-id <tenancy_ocid> --service-name compute
      oci limits definition list --compartment-id <tenancy_ocid> --service-name container-engine

    The statement's quota family must be the limit's "supported-quota-families"
    value, NOT its "service-name": standard-e4-core-count belongs to service
    "compute" but quota family "compute-core", so the statement reads "set
    compute-core quota standard-e4-core-count ...". Check "are-quotas-supported"
    (true) rather than "is-quota-managed" (unpopulated in this API version).

    Reference a nested compartment by its path from the tenancy root, e.g.
    "in compartment HyperFleet:hyperfleet-ci" — a bare compartment name only
    resolves for direct children of the tenancy root.

    If container-engine does not expose a cluster-count quota, enforce the
    concurrent-cluster cap as a soft check in the sweep function instead
    (see functions/oci-ci-sweep) and leave this list to compute cores only.

    For the rhelcert tenancy specifically, both are already confirmed (see
    terraform/oci/ci.tfvars.example and terraform/oci/README.md) — Container
    Engine does expose "container-engine" / "cluster-count" there.
  EOT
  type        = list(string)

  validation {
    condition     = length(var.statements) > 0
    error_message = "At least one quota statement is required; see the variable description for how to derive it."
  }
}

variable "freeform_tags" {
  description = "Freeform tags applied to the quota policy."
  type        = map(string)
  default     = {}
}
