resource "oci_resource_scheduler_schedule" "sweep" {
  compartment_id = var.compartment_id
  display_name   = "hyperfleet-ci-sweep-schedule"
  description    = "Invokes the oci-ci-sweep function on a cron schedule."

  action             = "START_RESOURCE"
  recurrence_type    = "CRON"
  recurrence_details = var.schedule_recurrence

  resources {
    id = oci_functions_function.sweep.id
  }

  freeform_tags = var.freeform_tags
}
