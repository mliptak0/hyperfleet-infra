resource "oci_budget_budget" "ci" {
  compartment_id = var.tenancy_ocid
  amount         = var.amount
  reset_period   = "MONTHLY"
  display_name   = var.display_name
  description    = "Monthly spend cap for the hyperfleet-ci compartment (ephemeral e2e cluster runs)."

  target_type = "COMPARTMENT"
  targets     = [var.target_compartment_id]

  freeform_tags = var.freeform_tags
}

locals {
  recipients = join(",", var.alert_recipients)
}

resource "oci_budget_alert_rule" "actual" {
  for_each = toset([for t in var.alert_thresholds_percent : tostring(t)])

  budget_id      = oci_budget_budget.ci.id
  type           = "ACTUAL"
  threshold_type = "PERCENTAGE"
  threshold      = tonumber(each.value)
  display_name   = "hyperfleet-ci-actual-${each.value}pct"
  message        = "hyperfleet-ci compartment spend has reached ${each.value}% of its ${var.amount}/month budget."
  recipients     = local.recipients
}

resource "oci_budget_alert_rule" "forecast" {
  budget_id      = oci_budget_budget.ci.id
  type           = "FORECAST"
  threshold_type = "PERCENTAGE"
  threshold      = var.forecast_threshold_percent
  display_name   = "hyperfleet-ci-forecast-${var.forecast_threshold_percent}pct"
  message        = "hyperfleet-ci compartment is forecast to reach ${var.forecast_threshold_percent}% of its ${var.amount}/month budget before period end."
  recipients     = local.recipients
}
