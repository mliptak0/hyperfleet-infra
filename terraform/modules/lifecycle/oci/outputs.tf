output "application_id" {
  description = "OCID of the Functions Application hosting the sweep function."
  value       = oci_functions_application.sweep.id
}

output "function_id" {
  description = "OCID of the sweep function."
  value       = oci_functions_function.sweep.id
}

output "container_repository_path" {
  description = "OCIR repository path to push the sweep function's image to. Push by tag and set function_image to that tag reference (OCI Functions rejects a digest reference there), then pin function_image_digest to the pushed image's immutable sha256 digest — repository immutability isn't supported in us-sanjose-1, so that digest is the guard against the deployed content being silently swapped."
  value       = "${oci_artifacts_container_repository.sweep.namespace}/${oci_artifacts_container_repository.sweep.display_name}"
}

output "schedule_id" {
  description = "OCID of the Resource Scheduler schedule invoking the sweep function."
  value       = oci_resource_scheduler_schedule.sweep.id
}
