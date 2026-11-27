# Authenticate a Scalr run through workload identity federation instead of a
# long-lived token. Scalr sets TF_VAR_SCALR_OIDC_TOKEN to a fresh OIDC token
# in each run phase. Declaring the variable ephemeral (Terraform 1.10 or later)
# keeps the token out of the saved plan, so the apply phase exchanges its own.
variable "SCALR_OIDC_TOKEN" {
  type      = string
  sensitive = true
  ephemeral = true
}

provider "anthropic" {
  federation_rule_id = "fdrl_xxxxx"
  organization_id    = "00000000-0000-0000-0000-000000000000"
  service_account_id = "svac_xxxxx"
  identity_token     = var.SCALR_OIDC_TOKEN
}
