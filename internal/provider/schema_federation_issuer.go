package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func federationIssuerSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manage a workload identity federation issuer using OIDC discovery or an explicit JWKS URL. Requires an OAuth bearer token with `org:admin`; Admin API keys are not accepted. Destroy archives the issuer. Inline signing keys, custom CAs and custom discovery base URLs are not supported.",
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "ID of the federation issuer."},
			"name":                     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 255)}, MarkdownDescription: "Unique slug identifier (lowercase, digits and hyphens)."},
			"issuer_url":               schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Exact JWT issuer (`iss`) value."},
			"jwks_url":                 schema.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Explicit JWKS endpoint. Omit to use OIDC discovery at the issuer URL."},
			"check_jti":                schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Enforce single-use for assertions carrying a `jti` claim."},
			"max_jwt_lifetime_seconds": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(3600), Validators: []validator.Int64{int64validator.Between(1, 176400)}, MarkdownDescription: "Maximum assertion lifetime in seconds."},
		},
	}
}
