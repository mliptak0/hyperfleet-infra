module "ci_compartment" {
  source                = "../modules/compartment/oci"
  parent_compartment_id = var.team_compartment_id
  freeform_tags         = local.tags
}

module "ci_quota" {
  source       = "../modules/quota/oci"
  tenancy_ocid = var.tenancy_ocid
  statements   = var.quota_statements

  freeform_tags = local.tags

  depends_on = [module.ci_compartment]
}

module "ci_budget" {
  source                = "../modules/budget/oci"
  tenancy_ocid          = var.tenancy_ocid
  target_compartment_id = module.ci_compartment.id
  amount                = var.budget_amount
  alert_recipients      = var.budget_alert_recipients

  freeform_tags = local.tags
}

module "ci_sweep" {
  source                = "../modules/lifecycle/oci"
  tenancy_ocid          = var.tenancy_ocid
  compartment_id        = module.ci_compartment.id
  function_image        = var.sweep_function_image
  function_image_digest = var.sweep_function_image_digest

  run_window_hours    = var.sweep_run_window_hours
  dry_run             = var.sweep_dry_run
  schedule_recurrence = var.sweep_schedule_recurrence

  freeform_tags = local.tags
}

locals {
  tags = {
    "hyperfleet-managed-by" = "terraform"
    "hyperfleet-purpose"    = "ci"
  }
}
