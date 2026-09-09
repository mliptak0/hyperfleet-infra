# Dedicated, minimal network for the sweep function's Application. OCI
# Functions always run inside a VCN subnet even though they're serverless.
# The subnet is private: the function only needs to reach other OCI services
# (Container Engine, Load Balancer, Block Storage, Database APIs), which it
# does over the OCI backbone via a service gateway, so no NAT/internet egress
# is required.

# The service gateway must target the "all OCI services in the region"
# destination so the function can reach every API it sweeps (Container Engine,
# Load Balancer, Block Storage, Database). oci_core_services returns several
# entries in a non-deterministic order, so filter for the aggregate one by
# name instead of indexing an arbitrary element.
data "oci_core_services" "all_oci_services" {
  filter {
    name   = "name"
    values = ["All .* Services In Oracle Services Network"]
    regex  = true
  }
}

resource "oci_core_vcn" "sweep" {
  compartment_id = var.compartment_id
  display_name   = "hyperfleet-ci-sweep-vcn"
  cidr_block     = var.vcn_cidr_block
  freeform_tags  = var.freeform_tags
}

resource "oci_core_service_gateway" "sweep" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.sweep.id
  display_name   = "hyperfleet-ci-sweep-sgw"

  services {
    service_id = data.oci_core_services.all_oci_services.services[0].id
  }

  freeform_tags = var.freeform_tags
}

resource "oci_core_route_table" "sweep" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.sweep.id
  display_name   = "hyperfleet-ci-sweep-rt"

  route_rules {
    destination       = data.oci_core_services.all_oci_services.services[0].cidr_block
    destination_type  = "SERVICE_CIDR_BLOCK"
    network_entity_id = oci_core_service_gateway.sweep.id
  }

  freeform_tags = var.freeform_tags
}

resource "oci_core_security_list" "sweep" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.sweep.id
  display_name   = "hyperfleet-ci-sweep-sl"

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  freeform_tags = var.freeform_tags
}

resource "oci_core_subnet" "sweep" {
  compartment_id             = var.compartment_id
  vcn_id                     = oci_core_vcn.sweep.id
  display_name               = "hyperfleet-ci-sweep-subnet"
  cidr_block                 = var.subnet_cidr_block
  prohibit_public_ip_on_vnic = true
  route_table_id             = oci_core_route_table.sweep.id
  security_list_ids          = [oci_core_security_list.sweep.id]

  freeform_tags = var.freeform_tags
}
