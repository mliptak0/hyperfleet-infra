# Immutable repositories would keep whoever can push to OCIR from silently
# swapping the image a scheduled, already-deployed function runs (the
# function's IAM grants it manage access on cluster/LB/volume/DB resources in
# this compartment). However, the Artifacts API in us-sanjose-1 rejects
# isImmutable with "400-BAD_REQUEST, Setting isImmutable is not currently
# supported" (confirmed live 2026-09-04), so we cannot set it here. Instead the
# function is pinned by an immutable @sha256 digest via image_digest below: a
# digest is content-addressed, so unlike a tag it cannot be repointed to
# different content on a mutable repository.
resource "oci_artifacts_container_repository" "sweep" {
  compartment_id = var.compartment_id
  display_name   = "oci-ci-sweep"

  freeform_tags = var.freeform_tags
}

resource "oci_functions_application" "sweep" {
  compartment_id = var.compartment_id
  display_name   = "oci-ci-sweep"
  subnet_ids     = [oci_core_subnet.sweep.id]

  freeform_tags = var.freeform_tags
}

resource "oci_functions_function" "sweep" {
  application_id = oci_functions_application.sweep.id
  display_name   = "oci-ci-sweep"
  # OCI Functions requires image to be a tag reference (repo:tag); it rejects a
  # digest reference (repo@sha256:...) with "Image is not a valid docker image
  # name". The digest pin lives in the separate image_digest field, which is
  # what OCI actually pulls by — so the tag is just a label and can move without
  # changing what runs.
  image         = var.function_image
  image_digest  = var.function_image_digest
  memory_in_mbs = "256"

  timeout_in_seconds = 300

  config = {
    COMPARTMENT_ID   = var.compartment_id
    RUN_WINDOW_HOURS = tostring(var.run_window_hours)
    DRY_RUN          = tostring(var.dry_run)
  }

  freeform_tags = var.freeform_tags
}
