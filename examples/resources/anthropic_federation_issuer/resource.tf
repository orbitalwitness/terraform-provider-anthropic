resource "anthropic_federation_issuer" "github" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}
