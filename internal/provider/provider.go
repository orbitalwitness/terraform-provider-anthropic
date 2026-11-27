package provider

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jianyuan/terraform-provider-anthropic/internal/providerdata"
)

// Ensure AnthropicProvider satisfies various provider interfaces.
var _ provider.Provider = &AnthropicProvider{}
var _ provider.ProviderWithFunctions = &AnthropicProvider{}

// AnthropicProvider defines the provider implementation.
type AnthropicProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// AnthropicProviderModel describes the provider data model.
type AnthropicProviderModel struct {
	BaseUrl           types.String `tfsdk:"base_url"`
	ApiKey            types.String `tfsdk:"api_key"`
	AuthToken         types.String `tfsdk:"auth_token"`
	FederationRuleId  types.String `tfsdk:"federation_rule_id"`
	OrganizationId    types.String `tfsdk:"organization_id"`
	ServiceAccountId  types.String `tfsdk:"service_account_id"`
	WorkspaceId       types.String `tfsdk:"workspace_id"`
	IdentityToken     types.String `tfsdk:"identity_token"`
	IdentityTokenFile types.String `tfsdk:"identity_token_file"`
}

func (p *AnthropicProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anthropic"
	resp.Version = p.version
}

func (p *AnthropicProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Anthropic provider is used to interact with the Anthropic service.\n\nIf you find this provider useful, please consider supporting me through GitHub Sponsorship or Ko-Fi to help with its development.\n\n[![Github-sponsors](https://img.shields.io/badge/sponsor-30363D?style=for-the-badge&logo=GitHub-Sponsors&logoColor=#EA4AAA)](https://github.com/sponsors/jianyuan)\n[![Ko-Fi](https://img.shields.io/badge/Ko--fi-F16061?style=for-the-badge&logo=ko-fi&logoColor=white)](https://ko-fi.com/L3L71DQEL)",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				MarkdownDescription: "API endpoint for the Anthropic service. Defaults to `https://api.anthropic.com`. It can be sourced from the `ANTHROPIC_BASE_URL` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The Admin API key for authentication. Get this from the [Anthropic console](https://console.anthropic.com/settings/admin-keys). It can be sourced from the `ANTHROPIC_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"auth_token": schema.StringAttribute{
				MarkdownDescription: "The OAuth bearer token for authentication. Log in with the `ant` CLI under a dedicated profile with the `org:admin` scope (see [Admin access](https://platform.claude.com/docs/en/cli-sdks-libraries/cli/authentication#admin-access)), then export the bearer token. It can be sourced from the `ANTHROPIC_AUTH_TOKEN` environment variable. Conflicts with workload identity federation.",
				Optional:            true,
				Sensitive:           true,
			},
			"federation_rule_id": schema.StringAttribute{
				MarkdownDescription: "The ID (`fdrl_...`) of the federation rule used to exchange `identity_token` or `identity_token_file` for a short-lived OAuth token through workload identity federation. Replaces `auth_token`. It can be sourced from the `ANTHROPIC_FEDERATION_RULE_ID` environment variable.",
				Optional:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The UUID of the Anthropic organisation the federation rule belongs to. Required with `federation_rule_id`. It can be sourced from the `ANTHROPIC_ORGANIZATION_ID` environment variable.",
				Optional:            true,
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "The ID (`svac_...`) of the service account a service-account federation rule targets. Optional; the exchange fails if the rule targets a different service account. It can be sourced from the `ANTHROPIC_SERVICE_ACCOUNT_ID` environment variable.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "The ID (`wrkspc_...`), or `default`, of the workspace the exchanged token is scoped to. Required only when the federation rule covers more than one workspace. It can be sourced from the `ANTHROPIC_WORKSPACE_ID` environment variable.",
				Optional:            true,
			},
			"identity_token": schema.StringAttribute{
				MarkdownDescription: "The OIDC identity token (JWT) to exchange, such as Scalr's `SCALR_OIDC_TOKEN`. Pass it through an ephemeral variable so that each run phase uses its own token. The provider re-exchanges the same token shortly before the minted OAuth token expires, which the issuer rejects once the token has expired or, with `check_jti`, been used, so keep runs shorter than the minted token's lifetime. Conflicts with `identity_token_file`. It can be sourced from the `ANTHROPIC_IDENTITY_TOKEN` environment variable, which is re-read on every exchange.",
				Optional:            true,
				Sensitive:           true,
			},
			"identity_token_file": schema.StringAttribute{
				MarkdownDescription: "The path to a file holding the OIDC identity token (JWT) to exchange. The file is re-read on every exchange, so a rotated token is picked up. Conflicts with `identity_token`. It can be sourced from the `ANTHROPIC_IDENTITY_TOKEN_FILE` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *AnthropicProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data AnthropicProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	options := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
	}

	var baseUrl string
	if !data.BaseUrl.IsNull() {
		options = append(options, option.WithBaseURL(data.BaseUrl.ValueString()))
		baseUrl = data.BaseUrl.ValueString()
	} else if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		options = append(options, option.WithBaseURL(v))
		baseUrl = v
	} else {
		baseUrl = "https://api.anthropic.com"
	}

	if baseUrl == "" {
		resp.Diagnostics.AddError("base_url is required", "base_url is required")
		return
	}

	var apiKey string
	if !data.ApiKey.IsNull() {
		apiKey = data.ApiKey.ValueString()
	} else if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		apiKey = v
	}

	if apiKey != "" && !strings.HasPrefix(apiKey, "sk-ant-admin") {
		resp.Diagnostics.AddError("api_key is invalid", "api_key is invalid. It must start with 'sk-ant-admin'.")
		return
	}

	var authToken string
	if !data.AuthToken.IsNull() {
		authToken = data.AuthToken.ValueString()
	} else if v := os.Getenv("ANTHROPIC_AUTH_TOKEN"); v != "" {
		authToken = v
	}

	if authToken != "" && !strings.HasPrefix(authToken, "sk-ant-oat") {
		resp.Diagnostics.AddError("auth_token is invalid", "auth_token is invalid. It must start with 'sk-ant-oat'.")
		return
	}

	federation, ok := federationOption(&data, &resp.Diagnostics)
	if !ok {
		return
	}
	if federation != nil && authToken != "" {
		resp.Diagnostics.AddError("Conflicting authentication", "auth_token cannot be combined with workload identity federation. Remove auth_token (or ANTHROPIC_AUTH_TOKEN), or the federation settings.")
		return
	}

	options = append(options,
		option.WithHTTPClient(&http.Client{Transport: newRedactingTransport(http.DefaultTransport)}),
		option.WithMaxRetries(5),
	)

	apiKeyClient := anthropic.NewClient(append(options, option.WithAPIKey(apiKey))...)
	authTokenCredential := option.WithAuthToken(authToken)
	if federation != nil {
		authTokenCredential = federation
	}
	authTokenClient := anthropic.NewClient(append(options, authTokenCredential)...)

	providerData := &providerdata.ProviderData{
		ApiKey:          apiKey,
		AuthToken:       authToken,
		ApiKeyClient:    &apiKeyClient,
		AuthTokenClient: &authTokenClient,
	}

	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *AnthropicProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewFederationIssuerResource,
		NewFederationRuleResource,
		NewOrganizationInviteResource,
		NewServiceAccountResource,
		NewServiceAccountWorkspaceMemberResource,
		NewWorkspaceMemberResource,
		NewWorkspaceResource,
	}
}

func (p *AnthropicProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewOrganizationInvitesDataSource,
		NewServiceAccountDataSource,
		NewServiceAccountsDataSource,
		NewUserDataSource,
		NewUsersDataSource,
		NewWorkspaceDataSource,
		NewWorkspaceMemberDataSource,
		NewWorkspaceMembersDataSource,
		NewWorkspaceRateLimitsDataSource,
		NewWorkspacesDataSource,
	}
}

func (p *AnthropicProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AnthropicProvider{
			version: version,
		}
	}
}
