variable "tenancy_ocid" {
  description = "Tenancy OCID. The budget's own compartment_id must be the tenancy; target_compartment_id scopes what spend it tracks."
  type        = string
}

variable "target_compartment_id" {
  description = "OCID of the compartment whose spend this budget tracks (the CI compartment)."
  type        = string
}

variable "display_name" {
  description = "Display name for the budget."
  type        = string
  default     = "hyperfleet-ci-budget"
}

variable "amount" {
  description = "Monthly budget amount, in the tenancy's currency."
  type        = number
  default     = 150
}

variable "alert_recipients" {
  description = <<-EOT
    Email addresses that receive budget alerts, via the alert rule's built-in
    email delivery (no Slack app/webhook setup required).
  EOT
  type        = list(string)

  validation {
    condition     = length(var.alert_recipients) > 0
    error_message = "At least one alert recipient email is required."
  }
}

variable "alert_thresholds_percent" {
  description = "Percent-of-budget thresholds (of ACTUAL spend) that trigger an alert."
  type        = list(number)
  default     = [50, 80, 100]
}

variable "forecast_threshold_percent" {
  description = "Percent-of-budget threshold on FORECASTED spend that triggers an early-warning alert."
  type        = number
  default     = 100
}

variable "freeform_tags" {
  description = "Freeform tags applied to the budget."
  type        = map(string)
  default     = {}
}
