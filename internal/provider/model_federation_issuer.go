package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jianyuan/terraform-provider-anthropic/internal/fwtypes"
)

type FederationIssuerModel struct {
	Id                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	IssuerURL             types.String `tfsdk:"issuer_url"`
	JWKSURL               types.String `tfsdk:"jwks_url"`
	CheckJTI              types.Bool   `tfsdk:"check_jti"`
	MaxJWTLifetimeSeconds types.Int64  `tfsdk:"max_jwt_lifetime_seconds"`
}

func (m *FederationIssuerModel) FromAPI(ctx context.Context, item anthropic.FederationIssuer) diag.Diagnostics {
	m.Id = types.StringValue(item.ID)
	m.Name = types.StringValue(item.Name)
	m.IssuerURL = types.StringValue(item.IssuerURL)
	m.CheckJTI = types.BoolValue(item.CheckJTI)
	m.MaxJWTLifetimeSeconds = types.Int64Value(item.MaxJWTLifetimeSeconds)
	m.JWKSURL = types.StringNull()
	if item.JWKS.Type == "explicit_url" {
		m.JWKSURL = types.StringValue(item.JWKS.URL)
	}
	var diags diag.Diagnostics
	if item.JWKS.Type != "discovery" && item.JWKS.Type != "explicit_url" {
		diags.AddError("Unsupported issuer JWKS mode", "This resource supports OIDC discovery and explicit JWKS URLs. Inline signing keys are not supported.")
	}
	if item.JWKS.CACertPEM != "" || item.JWKS.DiscoveryBase != "" {
		diags.AddError("Unsupported issuer JWKS configuration", "Custom CA certificates and discovery base URLs are not supported by this resource.")
	}
	return diags
}

func (m *FederationIssuerModel) ToAPIForCreate(ctx context.Context) (*anthropic.OrganizationFederationIssuerNewParams, diag.Diagnostics) {
	body := &anthropic.OrganizationFederationIssuerNewParams{
		Name: m.Name.ValueString(), IssuerURL: m.IssuerURL.ValueString(),
		CheckJTI:              anthropic.Bool(m.CheckJTI.ValueBool()),
		MaxJWTLifetimeSeconds: anthropic.Int(m.MaxJWTLifetimeSeconds.ValueInt64()),
	}
	if fwtypes.IsKnown(m.JWKSURL) {
		body.JWKS.OfExplicitURL = &anthropic.JWKSExplicitURLParam{URL: m.JWKSURL.ValueString()}
	} else {
		body.JWKS.OfDiscovery = &anthropic.JWKSDiscoveryParam{}
	}
	return body, nil
}

func (m *FederationIssuerModel) ToAPIForUpdate(ctx context.Context) (*anthropic.OrganizationFederationIssuerUpdateParams, diag.Diagnostics) {
	body := &anthropic.OrganizationFederationIssuerUpdateParams{
		Name: anthropic.String(m.Name.ValueString()), IssuerURL: anthropic.String(m.IssuerURL.ValueString()),
		CheckJTI:              anthropic.Bool(m.CheckJTI.ValueBool()),
		MaxJWTLifetimeSeconds: anthropic.Int(m.MaxJWTLifetimeSeconds.ValueInt64()),
	}
	if fwtypes.IsKnown(m.JWKSURL) {
		body.JWKS.OfExplicitURL = &anthropic.JWKSExplicitURLParam{URL: m.JWKSURL.ValueString()}
	} else {
		body.JWKS.OfDiscovery = &anthropic.JWKSDiscoveryParam{}
	}
	return body, nil
}
