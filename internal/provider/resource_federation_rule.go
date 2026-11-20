package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/jianyuan/terraform-provider-anthropic/internal/fwresource"
)

var _ resource.Resource = &FederationRuleResource{}
var _ resource.ResourceWithConfigure = &FederationRuleResource{}
var _ resource.ResourceWithImportState = &FederationRuleResource{}

func NewFederationRuleResource() resource.Resource {
	return &FederationRuleResource{}
}

type FederationRuleResource struct {
	baseResource
}

func (r *FederationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule"
}

func (r *FederationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = federationRuleSchema()
}

func (r *FederationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	fwresource.Create(ctx, func(_ *FederationRuleModel, body *anthropic.OrganizationFederationRuleNewParams) (*anthropic.FederationRule, error) {
		defer invalidateList(r.authTokenClient, "federation_rule")
		return r.authTokenClient.Organization.Federation.Rules.New(ctx, *body)
	}, req, resp)
}

func (r *FederationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwresource.Read(ctx, func(data *FederationRuleModel) (*anthropic.FederationRule, error) {
		return cachedGet(ctx, r.authTokenClient, "federation_rule", data.Id.ValueString(), func(ctx context.Context) (map[string]anthropic.FederationRule, error) {
			items := map[string]anthropic.FederationRule{}
			iter := r.authTokenClient.Organization.Federation.Rules.ListAutoPaging(ctx, anthropic.OrganizationFederationRuleListParams{})
			for iter.Next() {
				items[iter.Current().ID] = iter.Current()
			}
			return items, iter.Err()
		})
	}, req, resp)
}

func (r *FederationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	fwresource.Update(ctx, func(data *FederationRuleModel, body *anthropic.OrganizationFederationRuleUpdateParams) (*anthropic.FederationRule, error) {
		defer invalidateList(r.authTokenClient, "federation_rule")
		return r.authTokenClient.Organization.Federation.Rules.Update(ctx, data.Id.ValueString(), *body)
	}, req, resp)
}

func (r *FederationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	fwresource.Delete(ctx, func(data *FederationRuleModel) error {
		defer invalidateList(r.authTokenClient, "federation_rule")
		_, err := r.authTokenClient.Organization.Federation.Rules.Archive(ctx, data.Id.ValueString())
		return err
	}, req, resp)
}

func (r *FederationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
