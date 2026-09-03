resource "oci_limits_quota" "this" {
  compartment_id = var.tenancy_ocid
  name           = var.name
  description    = var.description
  statements     = var.statements
  freeform_tags  = var.freeform_tags
}
