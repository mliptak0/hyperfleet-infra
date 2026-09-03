resource "oci_logging_log_group" "sweep" {
  compartment_id = var.compartment_id
  display_name   = "oci-ci-sweep"

  freeform_tags = var.freeform_tags
}

# Invocation logs for the sweep function: without this, the scheduled runs'
# per-resource slog output and JSON summary are written to stdout and dropped,
# leaving no record of what the sweep evaluated or deleted.
resource "oci_logging_log" "sweep_invoke" {
  display_name = "oci-ci-sweep-invoke"
  log_group_id = oci_logging_log_group.sweep.id
  log_type     = "SERVICE"

  configuration {
    compartment_id = var.compartment_id

    source {
      category    = "invoke"
      resource    = oci_functions_application.sweep.id
      service     = "functions"
      source_type = "OCISERVICE"
    }
  }

  is_enabled         = true
  retention_duration = var.log_retention_days
}
