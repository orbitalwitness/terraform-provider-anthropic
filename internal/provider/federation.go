package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
)

const (
	envFederationRuleId  = "ANTHROPIC_FEDERATION_RULE_ID"
	envOrganizationId    = "ANTHROPIC_ORGANIZATION_ID"
	envServiceAccountId  = "ANTHROPIC_SERVICE_ACCOUNT_ID"
	envWorkspaceId       = "ANTHROPIC_WORKSPACE_ID"
	envIdentityToken     = "ANTHROPIC_IDENTITY_TOKEN"
	envIdentityTokenFile = "ANTHROPIC_IDENTITY_TOKEN_FILE"

	tokenEndpointPath = "/v1/oauth/token"
)

func attrOrEnv(v types.String, env string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	return os.Getenv(env)
}

func attrSet(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}

// identityTokenSource returns nil when no identity token is configured.
func identityTokenSource(data *AnthropicProviderModel) (option.IdentityTokenFunc, error) {
	switch {
	case attrSet(data.IdentityToken) && attrSet(data.IdentityTokenFile):
		return nil, errors.New("identity_token and identity_token_file cannot both be set")
	case attrSet(data.IdentityToken):
		token := strings.TrimSpace(data.IdentityToken.ValueString())
		return func(context.Context) (string, error) { return token, nil }, nil
	case attrSet(data.IdentityTokenFile):
		return option.IdentityTokenFile(data.IdentityTokenFile.ValueString()), nil
	case os.Getenv(envIdentityTokenFile) != "":
		return option.IdentityTokenFile(os.Getenv(envIdentityTokenFile)), nil
	case os.Getenv(envIdentityToken) != "":
		return func(context.Context) (string, error) {
			token := strings.TrimSpace(os.Getenv(envIdentityToken))
			if token == "" {
				return "", fmt.Errorf("%s is empty", envIdentityToken)
			}
			return token, nil
		}, nil
	}
	return nil, nil
}

// federationOption returns the workload identity federation credential, or
// nil when federation is not configured. ok is false when diagnostics were
// added.
func federationOption(data *AnthropicProviderModel, diags *diag.Diagnostics) (_ option.RequestOption, ok bool) {
	ruleId := attrOrEnv(data.FederationRuleId, envFederationRuleId)
	orgId := attrOrEnv(data.OrganizationId, envOrganizationId)
	serviceAccountId := attrOrEnv(data.ServiceAccountId, envServiceAccountId)
	workspaceId := attrOrEnv(data.WorkspaceId, envWorkspaceId)

	identityToken, err := identityTokenSource(data)
	if err != nil {
		diags.AddError("Invalid workload identity federation configuration", err.Error())
		return nil, false
	}

	explicit := attrSet(data.FederationRuleId) || attrSet(data.OrganizationId) || attrSet(data.ServiceAccountId) ||
		attrSet(data.WorkspaceId) || attrSet(data.IdentityToken) || attrSet(data.IdentityTokenFile)
	if ruleId == "" && identityToken == nil && !explicit {
		return nil, true
	}

	var missing []string
	if ruleId == "" {
		missing = append(missing, fmt.Sprintf("federation_rule_id (or %s)", envFederationRuleId))
	}
	if orgId == "" {
		missing = append(missing, fmt.Sprintf("organization_id (or %s)", envOrganizationId))
	}
	if identityToken == nil {
		missing = append(missing, fmt.Sprintf("identity_token or identity_token_file (or %s / %s)", envIdentityToken, envIdentityTokenFile))
	}
	if len(missing) > 0 {
		diags.AddError("Incomplete workload identity federation configuration",
			"Workload identity federation needs federation_rule_id, organization_id and an identity token set together. Missing: "+strings.Join(missing, ", ")+".")
		return nil, false
	}

	for _, check := range []struct {
		name, value, prefix string
		allowed             []string
	}{
		{"federation_rule_id", ruleId, "fdrl_", nil},
		{"service_account_id", serviceAccountId, "svac_", nil},
		{"workspace_id", workspaceId, "wrkspc_", []string{"default"}},
	} {
		if check.value == "" || strings.HasPrefix(check.value, check.prefix) || slices.Contains(check.allowed, check.value) {
			continue
		}
		diags.AddError(check.name+" is invalid", fmt.Sprintf("%s is invalid. It must start with '%s'.", check.name, check.prefix))
	}
	if diags.HasError() {
		return nil, false
	}

	return option.WithFederationTokenProvider(identityToken, option.FederationOptions{
		FederationRuleID: ruleId,
		OrganizationID:   orgId,
		ServiceAccountID: serviceAccountId,
		WorkspaceID:      workspaceId,
	}), true
}

// redactingTransport keeps credentials out of the debug HTTP log: it masks
// credential headers and never logs token exchanges, whose bodies carry the
// identity token and the minted access token.
type redactingTransport struct {
	logged, unlogged http.RoundTripper
}

func newRedactingTransport(t http.RoundTripper) http.RoundTripper {
	return &redactingTransport{logged: logging.NewLoggingHTTPTransport(t), unlogged: t}
}

func (t *redactingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, tokenEndpointPath) {
		return t.unlogged.RoundTrip(req)
	}
	ctx := tflog.MaskFieldValuesWithFieldKeys(req.Context(), "Authorization", "X-Api-Key")
	return t.logged.RoundTrip(req.WithContext(ctx))
}
