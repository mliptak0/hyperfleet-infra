resource "oci_identity_compartment" "this" {
  compartment_id = var.parent_compartment_id
  name           = var.name
  description    = var.description
  freeform_tags  = var.freeform_tags

  enable_delete = true
}
