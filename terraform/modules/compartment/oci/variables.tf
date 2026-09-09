variable "parent_compartment_id" {
  description = <<-EOT
    OCID of the HyperFleet team compartment. hyperfleet-ci is created directly
    under it, as a sibling of hyperfleet-sandbox/hyperfleet-poc/hyperfleet-demos
    — never point this at one of those sub-compartments, and never create
    resources directly in the team compartment itself (team convention: every
    resource lives in a sub-compartment so spend is attributable).
  EOT
  type        = string
}

variable "name" {
  description = "Name of the compartment."
  type        = string
  default     = "hyperfleet-ci"
}

variable "description" {
  description = "Description of the compartment."
  type        = string
  default     = "HyperFleet OCI CI compartment: ephemeral e2e cluster runs, swept on a schedule."
}

variable "freeform_tags" {
  description = "Freeform tags applied to the compartment."
  type        = map(string)
  default     = {}
}
