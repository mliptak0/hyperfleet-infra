variable "tenancy_ocid" {
  description = "Tenancy OCID. Dynamic groups must be created at the tenancy root."
  type        = string
}

variable "compartment_id" {
  description = "OCID of the CI compartment the sweep function watches and cleans up."
  type        = string
}

variable "function_image" {
  description = <<-EOT
    Tag-form OCIR image reference for the sweep function, e.g.
    "<region-key>.ocir.io/<namespace>/oci-ci-sweep:<tag>", built and pushed out
    of band by CI from functions/oci-ci-sweep. OCI Functions requires a tag
    reference here and rejects a digest reference; the immutable pin is supplied
    separately via function_image_digest. The container_repository_path output
    gives the repository half of this reference.
  EOT
  type        = string

  validation {
    condition     = can(regex(":[^/@:]+$", var.function_image)) && !can(regex("@", var.function_image))
    error_message = "function_image must be a tag reference (repo:tag), not a digest reference (repo@sha256:...). The digest goes in function_image_digest."
  }
}

variable "function_image_digest" {
  description = <<-EOT
    Immutable @sha256 digest the function is pinned to, e.g.
    "sha256:<64-hex>", read back from OCIR after pushing the tag in
    function_image. This is what OCI Functions actually pulls by, so unlike the
    tag it cannot be repointed to different content: repository immutability
    isn't supported by the Artifacts API in us-sanjose-1, making this digest pin
    the guard against a deployed function's image being silently swapped.
  EOT
  type        = string

  validation {
    condition     = can(regex("^sha256:[0-9a-f]{64}$", var.function_image_digest))
    error_message = "function_image_digest must be an immutable digest of the form sha256:<64 hex chars>."
  }
}

variable "run_window_hours" {
  description = "Age, in hours, past which a resource in the CI compartment is swept."
  type        = number
  default     = 8

  validation {
    condition     = var.run_window_hours >= 1 && var.run_window_hours <= 8760 && floor(var.run_window_hours) == var.run_window_hours
    error_message = "run_window_hours must be a whole number between 1 and 8760 (1 year)."
  }
}

variable "dry_run" {
  description = "When true, the sweep logs what it would delete without deleting anything. Flip to false once verified."
  type        = bool
  default     = true
}

variable "schedule_recurrence" {
  description = "Cron expression controlling how often the sweep runs."
  type        = string
  default     = "0 * * * *"

  validation {
    condition     = can(regex("^[0-9*,/-]+([ \t]+[0-9*,/-]+){4}$", trimspace(var.schedule_recurrence)))
    error_message = "schedule_recurrence must be a five-field UNIX cron expression (minute hour day-of-month month day-of-week), e.g. \"0 * * * *\". OCI Resource Scheduler validates field ranges at apply time."
  }
}

variable "vcn_cidr_block" {
  description = "CIDR block for the dedicated VCN the sweep function's Application runs in."
  type        = string
  default     = "10.99.0.0/24"
}

variable "subnet_cidr_block" {
  description = "CIDR block for the private subnet the sweep function's Application runs in."
  type        = string
  default     = "10.99.0.0/25"
}

variable "log_retention_days" {
  description = "Retention in days for the sweep function's invocation logs. Must be one of OCI's allowed values (30, 60, 90, ... 360)."
  type        = number
  default     = 30

  validation {
    condition     = contains([30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330, 360], var.log_retention_days)
    error_message = "log_retention_days must be one of: 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330, 360."
  }
}

variable "freeform_tags" {
  description = "Freeform tags applied to created resources."
  type        = map(string)
  default     = {}
}
