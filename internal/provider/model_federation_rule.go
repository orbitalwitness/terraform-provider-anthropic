package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jianyuan/terraform-provider-anthropic/internal/fwtypes"
)

type FederationRuleModel struct {
	Id                   types.String              `tfsdk:"id"`
	Name                 types.String              `tfsdk:"name"`
	IssuerID             types.String              `tfsdk:"issuer_id"`
	ServiceAccountID     types.String              `tfsdk:"service_account_id"`
	WorkspaceID          types.String              `tfsdk:"workspace_id"`
	OAuthScope           types.String              `tfsdk:"oauth_scope"`
	Description          types.String              `tfsdk:"description"`
	TokenLifetimeSeconds types.Int64               `tfsdk:"token_lifetime_seconds"`
	Match                *FederationRuleMatchModel `tfsdk:"match"`
}

type FederationRuleMatchModel struct {
	Audience      types.String `tfsdk:"audience"`
	SubjectPrefix types.String `tfsdk:"subject_prefix"`
	Condition     types.String `tfsdk:"condition"`
	Claims        types.Map    `tfsdk:"claims"`
}

func (m *FederationRuleModel) FromAPI(ctx context.Context, item anthropic.FederationRule) diag.Diagnostics {
	var diags diag.Diagnostics
	if item.AppliesToAllWorkspaces || len(item.WorkspaceIDs) != 1 {
		diags.AddError("Unsupported workspace targeting", "This resource manages a rule enabled for exactly one workspace.")
		return diags
	}
	m.Id = types.StringValue(item.ID)
	m.Name = types.StringValue(item.Name)
	m.IssuerID = types.StringValue(item.IssuerID)
	m.ServiceAccountID = types.StringValue(item.Target.ServiceAccountID)
	m.WorkspaceID = types.StringValue(item.WorkspaceIDs[0])
	m.OAuthScope = types.StringValue(item.OAuthScope)
	m.TokenLifetimeSeconds = types.Int64Value(item.TokenLifetimeSeconds)
	// The API stores a cleared description as an empty string.
	if item.Description != "" || !m.Description.IsNull() {
		m.Description = types.StringValue(item.Description)
	}
	m.Match = &FederationRuleMatchModel{
		Audience:      fwtypes.StringValue(item.Match.Audience, item.Match.JSON.Audience, fwtypes.WithUnknownAsNull()),
		SubjectPrefix: fwtypes.StringValue(item.Match.SubjectPrefix, item.Match.JSON.SubjectPrefix, fwtypes.WithUnknownAsNull()),
		Condition:     fwtypes.StringValue(item.Match.Condition, item.Match.JSON.Condition, fwtypes.WithUnknownAsNull()),
		Claims:        types.MapNull(types.StringType),
	}
	if len(item.Match.Claims) > 0 {
		var d diag.Diagnostics
		m.Match.Claims, d = types.MapValueFrom(ctx, types.StringType, item.Match.Claims)
		diags.Append(d...)
	}
	return diags
}

func (m *FederationRuleModel) apiMatch(ctx context.Context) (anthropic.FederationRuleMatchParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	var match anthropic.FederationRuleMatchParam
	if m.Match == nil {
		diags.AddError("Missing match", "A federation rule must constrain the workload identity.")
		return match, diags
	}
	if fwtypes.IsKnown(m.Match.Audience) {
		match.Audience = anthropic.String(m.Match.Audience.ValueString())
	}
	if fwtypes.IsKnown(m.Match.SubjectPrefix) {
		match.SubjectPrefix = anthropic.String(m.Match.SubjectPrefix.ValueString())
	}
	if fwtypes.IsKnown(m.Match.Condition) {
		match.Condition = anthropic.String(m.Match.Condition.ValueString())
	}
	if fwtypes.IsKnown(m.Match.Claims) {
		diags.Append(m.Match.Claims.ElementsAs(ctx, &match.Claims, false)...)
	}
	if m.Match.SubjectPrefix.ValueString() == "" && m.Match.Condition.ValueString() == "" && len(match.Claims) == 0 {
		diags.AddError("Missing identity constraint", "Set match.subject_prefix, match.claims or match.condition; audience alone is not sufficient.")
	}
	return match, diags
}

func (m *FederationRuleModel) ToAPIForCreate(ctx context.Context) (*anthropic.OrganizationFederationRuleNewParams, diag.Diagnostics) {
	match, diags := m.apiMatch(ctx)
	body := &anthropic.OrganizationFederationRuleNewParams{
		Name: m.Name.ValueString(), IssuerID: m.IssuerID.ValueString(),
		OAuthScope: m.OAuthScope.ValueString(), Match: match,
		Target:               anthropic.ServiceAccountTargetParam{ServiceAccountID: m.ServiceAccountID.ValueString()},
		WorkspaceID:          anthropic.String(m.WorkspaceID.ValueString()),
		TokenLifetimeSeconds: anthropic.Int(m.TokenLifetimeSeconds.ValueInt64()),
	}
	if fwtypes.IsKnown(m.Description) {
		body.Description = anthropic.String(m.Description.ValueString())
	}
	return body, diags
}

func (m *FederationRuleModel) ToAPIForUpdate(ctx context.Context) (*anthropic.OrganizationFederationRuleUpdateParams, diag.Diagnostics) {
	match, diags := m.apiMatch(ctx)
	return &anthropic.OrganizationFederationRuleUpdateParams{
		Name: anthropic.String(m.Name.ValueString()), OAuthScope: anthropic.String(m.OAuthScope.ValueString()),
		Match: match, Target: anthropic.ServiceAccountTargetParam{ServiceAccountID: m.ServiceAccountID.ValueString()},
		WorkspaceID:          anthropic.String(m.WorkspaceID.ValueString()),
		Description:          anthropic.String(m.Description.ValueString()),
		TokenLifetimeSeconds: anthropic.Int(m.TokenLifetimeSeconds.ValueInt64()),
	}, diags
}
