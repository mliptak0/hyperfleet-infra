variable "tenancy_ocid" {
  description = "OCID of the rhelcert tenancy."
  type        = string
}

variable "region" {
  description = "OCI region to create resources in."
  type        = string
  default     = "us-sanjose-1"
}

variable "oci_auth" {
  description = "OCI provider auth mode: \"ApiKey\" (durable) or \"SecurityToken\" (browser SSO session, via `oci session authenticate`)."
  type        = string
  default     = "ApiKey"

  validation {
    condition     = contains(["ApiKey", "SecurityToken"], var.oci_auth)
    error_message = "oci_auth must be \"ApiKey\" or \"SecurityToken\"."
  }
}

variable "oci_config_file_profile" {
  description = "Profile name in ~/.oci/config to use when oci_auth = \"SecurityToken\"."
  type        = string
  default     = "DEFAULT"
}

variable "oci_user_ocid" {
  description = "User OCID, when oci_auth = \"ApiKey\"."
  type        = string
  default     = null
}

variable "oci_fingerprint" {
  description = "API key fingerprint, when oci_auth = \"ApiKey\"."
  type        = string
  default     = null
}

variable "oci_private_key_path" {
  description = "Path to the API private key, when oci_auth = \"ApiKey\"."
  type        = string
  default     = null
}

variable "team_compartment_id" {
  description = <<-EOT
    OCID of the "HyperFleet" team compartment (rhelcert tenancy). This is the
    parent, not one of its existing sub-compartments — hyperfleet-ci is
    created as a sibling of hyperfleet-sandbox/hyperfleet-poc/hyperfleet-demos,
    per team convention: never create resources directly in the team
    compartment, always in a sub-compartment.
  EOT
  type        = string
}

variable "quota_statements" {
  description = <<-EOT
    Quota policy statements. See terraform/modules/quota/oci/variables.tf for
    how to derive the correct compute-core and container-engine values for
    this tenancy before setting this.
  EOT
  type        = list(string)
}

variable "budget_amount" {
  description = "Monthly budget amount (USD) for the hyperfleet-ci compartment."
  type        = number
  default     = 150
}

variable "budget_alert_recipients" {
  description = "Email addresses that receive hyperfleet-ci budget alerts."
  type        = list(string)
}

variable "sweep_function_image" {
  description = <<-EOT
    Tag-form OCIR image reference for the oci-ci-sweep function (e.g.
    "sjc.ocir.io/<namespace>/oci-ci-sweep:<tag>"). OCI Functions requires a tag
    reference here; the immutable pin is set separately in
    sweep_function_image_digest. Push the image first, then set both.
  EOT
  type        = string

  validation {
    condition     = can(regex(":[^/@:]+$", var.sweep_function_image)) && !can(regex("@", var.sweep_function_image))
    error_message = "sweep_function_image must be a tag reference (repo:tag), not a digest reference (repo@sha256:...). The digest goes in sweep_function_image_digest."
  }
}

variable "sweep_function_image_digest" {
  description = <<-EOT
    Immutable @sha256 digest the oci-ci-sweep function is pinned to (e.g.
    "sha256:<64-hex>"). Read it back from OCIR after pushing the tag in
    sweep_function_image. Repository immutability isn't supported by the
    Artifacts API in us-sanjose-1, so this digest — not the tag — is what
    guarantees the deployed function keeps running the exact content reviewed.
  EOT
  type        = string

  validation {
    condition     = can(regex("^sha256:[0-9a-f]{64}$", var.sweep_function_image_digest))
    error_message = "sweep_function_image_digest must be an immutable digest of the form sha256:<64 hex chars>."
  }
}

variable "sweep_run_window_hours" {
  description = <<-EOT
    Age, in hours, past which the sweep deletes a resource. Keep this close
    to how long a normal e2e run actually takes plus a small buffer, not a
    full day: at $7/day per 3-node OKE cluster, a shorter window shrinks how
    much a single leaked/orphaned resource can cost before the sweep catches
    it (the sweep is the backstop for HYPERFLEET-1563's per-run teardown,
    not the primary cleanup path).
  EOT
  type        = number
  default     = 8

  validation {
    condition     = var.sweep_run_window_hours >= 1 && var.sweep_run_window_hours <= 8760 && floor(var.sweep_run_window_hours) == var.sweep_run_window_hours
    error_message = "sweep_run_window_hours must be a whole number between 1 and 8760 (1 year)."
  }
}

variable "sweep_dry_run" {
  description = "When true, the sweep only logs what it would delete. Flip to false once verified end to end."
  type        = bool
  default     = true
}

variable "sweep_schedule_recurrence" {
  description = "Cron expression for how often the sweep runs."
  type        = string
  default     = "0 * * * *"
}
