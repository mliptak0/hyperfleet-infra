# Dynamic group + policy #1: the sweep function's own runtime identity
# (resource principal). Scoped to this one function by OCID — not to every
# fnfunc in the compartment — so an unrelated function created here later
# doesn't silently inherit the sweep's manage grants. The function doesn't
# reference this group, so there's no dependency cycle.
resource "oci_identity_dynamic_group" "sweep_function" {
  compartment_id = var.tenancy_ocid
  name           = "hyperfleet-ci-sweep-fn"
  description    = "Matches the oci-ci-sweep function so it can authenticate as a resource principal."
  matching_rule  = "ALL {resource.type = 'fnfunc', resource.id = '${oci_functions_function.sweep.id}'}"

  freeform_tags = var.freeform_tags
}

resource "oci_identity_policy" "sweep_function" {
  compartment_id = var.compartment_id
  name           = "hyperfleet-ci-sweep-fn-policy"
  description    = "Lets the oci-ci-sweep function manage swept resource types, only in the CI compartment."

  statements = [
    "allow dynamic-group ${oci_identity_dynamic_group.sweep_function.name} to manage cluster-family in compartment id ${var.compartment_id}",
    "allow dynamic-group ${oci_identity_dynamic_group.sweep_function.name} to manage load-balancers in compartment id ${var.compartment_id}",
    "allow dynamic-group ${oci_identity_dynamic_group.sweep_function.name} to manage volume-family in compartment id ${var.compartment_id}",
    "allow dynamic-group ${oci_identity_dynamic_group.sweep_function.name} to manage database-family in compartment id ${var.compartment_id}",
  ]

  freeform_tags = var.freeform_tags
}

# Dynamic group + policy #2: the Resource Scheduler schedule itself, which
# needs permission to invoke the function on the sweep's cron cadence.
resource "oci_identity_dynamic_group" "scheduler" {
  compartment_id = var.tenancy_ocid
  name           = "hyperfleet-ci-sweep-scheduler"
  description    = "Matches the sweep's Resource Scheduler schedule so it can invoke the function."
  matching_rule  = "ALL {resource.type = 'resourceschedule', resource.id = '${oci_resource_scheduler_schedule.sweep.id}'}"

  freeform_tags = var.freeform_tags
}

resource "oci_identity_policy" "scheduler" {
  compartment_id = var.compartment_id
  name           = "hyperfleet-ci-sweep-scheduler-policy"
  description    = "Lets the sweep's Resource Scheduler schedule invoke the oci-ci-sweep function."

  statements = [
    "allow dynamic-group ${oci_identity_dynamic_group.scheduler.name} to manage functions-family in compartment id ${var.compartment_id}",
  ]

  freeform_tags = var.freeform_tags
}

# Policy #3: the FaaS (Functions) service itself, not a dynamic group. When a
# function runs, the service needs to plumb it into the VCN subnet and pull
# the image from OCIR on the function's behalf. Without these grants the
# function fails to invoke (network attach or image pull denied), regardless
# of what the function's own resource-principal policy allows.
#
# This policy is attached at the tenancy root because OCI requires the OCIR
# read grant for the FaaS service to be tenancy-scoped ("read repos in
# tenancy") — a compartment-attached policy cannot express "in tenancy", and
# scoping repos to a single compartment does not authorize the pull. The
# virtual-network-family statement is still scoped down to the CI compartment,
# where the subnet lives.
resource "oci_identity_policy" "faas_service" {
  compartment_id = var.tenancy_ocid
  name           = "hyperfleet-ci-faas-service-policy"
  description    = "Lets the FaaS service attach the sweep function to its subnet and pull its image."

  statements = [
    "allow service FaaS to use virtual-network-family in compartment id ${var.compartment_id}",
    "allow service FaaS to read repos in tenancy",
  ]

  freeform_tags = var.freeform_tags
}
