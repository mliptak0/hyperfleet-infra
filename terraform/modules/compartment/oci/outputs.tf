output "id" {
  description = "OCID of the CI compartment."
  value       = oci_identity_compartment.this.id
}

output "name" {
  description = "Name of the CI compartment."
  value       = oci_identity_compartment.this.name
}
