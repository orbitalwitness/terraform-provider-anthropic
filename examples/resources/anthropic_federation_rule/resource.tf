resource "anthropic_service_account" "deploy" {
  name = "ci-deploy"
}

resource "anthropic_service_account_workspace_member" "deploy" {
  service_account_id = anthropic_service_account.deploy.id
  workspace_id       = "wrkspc_example"
  workspace_role     = "workspace_developer"
}

resource "anthropic_federation_issuer" "github" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}

resource "anthropic_federation_rule" "deploy" {
  name               = "ci-deploy"
  issuer_id          = anthropic_federation_issuer.github.id
  service_account_id = anthropic_service_account.deploy.id
  workspace_id       = "wrkspc_example"
  oauth_scope        = "workspace:inference"
  match = {
    subject_prefix = "repo:example/app:ref:refs/heads/main"
  }
  depends_on = [anthropic_service_account_workspace_member.deploy]
}
