package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func federationRuleSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manage a workload identity federation rule enabled for one workspace. Requires an OAuth bearer token with `org:admin`; Admin API keys are not accepted. OAuth callers can manage only `workspace:developer` and `workspace:inference` rules. Bootstrap `org:admin` rules must be created in the Console. Destroy archives the rule. Multi-workspace and all-workspace targeting are not supported. Legacy rules without `workspace_ids` cannot be imported.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "ID of the federation rule."},
			"name":                   schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 255)}, MarkdownDescription: "Unique slug identifier (lowercase, digits and hyphens)."},
			"issuer_id":              schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, MarkdownDescription: "ID of the federation issuer. Changing it replaces the rule."},
			"service_account_id":     schema.StringAttribute{Required: true, MarkdownDescription: "ID of the target service account, which must be a member of the workspace."},
			"workspace_id":           schema.StringAttribute{Required: true, MarkdownDescription: "ID of the single workspace enabled for this rule."},
			"oauth_scope":            schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("workspace:developer", "workspace:inference")}, MarkdownDescription: "OAuth scope granted by this rule."},
			"description":            schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(2000)}, MarkdownDescription: "Optional description. Removing it clears the description."},
			"token_lifetime_seconds": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3600), Validators: []validator.Int64{int64validator.Between(60, 86400)}, MarkdownDescription: "Requested access-token lifetime in seconds, further capped by assertion validity."},
			"match": schema.SingleNestedAttribute{Required: true, MarkdownDescription: "JWT constraints. Set a subject, claims or CEL condition; audience alone is insufficient. Shared issuers must constrain tenant identity.", Attributes: map[string]schema.Attribute{
				"audience":       schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 1024)}, MarkdownDescription: "Exact audience match. Omit to use Anthropic's expected audience."},
				"subject_prefix": schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 1024), stringvalidator.NoneOf("*")}, MarkdownDescription: "Exact subject match, or prefix match if ending in `*`. A wildcard-only subject is rejected."},
				"condition":      schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 4096)}, MarkdownDescription: "CEL boolean expression over `claims`. Constant-true expressions are rejected by the API."},
				"claims":         schema.MapAttribute{Optional: true, ElementType: types.StringType, Validators: []validator.Map{mapvalidator.SizeAtLeast(1)}, MarkdownDescription: "Exact matches for top-level string claims."},
			}},
		},
	}
}
