terraform {
  required_providers {
    anthropic = {
      source  = "orbital.scalr.io/providers/anthropic"
      version = "= 0.4.0-rc"
    }
  }
}

# Provisioning uses ANTHROPIC_AUTH_TOKEN with org:admin, not the runtime token.
provider "anthropic" {}

variable "environment" {
  type = string
}
variable "workload" {
  type = string
}
variable "workspace_id" {
  type = string
}
variable "issuer_id" {
  type = string
}
variable "audience" {
  type = string
}
variable "subject" {
  type        = string
  description = "Exact verified workload subject. No wildcard matching."
}
variable "needs_files" {
  type        = bool
  default     = false
  description = "Explicitly allow the broader workspace:developer scope for Files/Skills. Current platform application keeps images inline."
}

resource "anthropic_service_account" "runtime" {
  name              = "${var.environment}-${var.workload}"
  organization_role = "developer"
}

resource "anthropic_service_account_workspace_member" "runtime" {
  workspace_id       = var.workspace_id
  service_account_id = anthropic_service_account.runtime.id
  workspace_role     = "workspace_restricted_developer"
}

resource "anthropic_federation_rule" "runtime" {
  name                   = "${var.environment}-${var.workload}"
  issuer_id              = var.issuer_id
  service_account_id     = anthropic_service_account.runtime.id
  workspace_id           = var.workspace_id
  oauth_scope            = var.needs_files ? "workspace:developer" : "workspace:inference"
  token_lifetime_seconds = 600
  match = {
    audience = var.audience
    claims   = { sub = var.subject }
  }
  depends_on = [anthropic_service_account_workspace_member.runtime]
}

output "runtime_identity" {
  value = {
    service_account_id = anthropic_service_account.runtime.id
    federation_rule_id = anthropic_federation_rule.runtime.id
    workspace_id       = var.workspace_id
  }
}
