package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/jianyuan/terraform-provider-anthropic/internal/apiclient"
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
		return r.authTokenClient.Organization.Federation.Rules.New(ctx, *body)
	}, req, resp)
}

func (r *FederationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FederationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.authTokenClient.Organization.Federation.Rules.Get(ctx, data.Id.ValueString())
	if err != nil {
		if apiclient.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Unable to read federation rule", err.Error())
		}
		return
	}
	if !item.ArchivedAt.IsZero() {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(data.FromAPI(ctx, *item)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *FederationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	fwresource.Update(ctx, func(data *FederationRuleModel, body *anthropic.OrganizationFederationRuleUpdateParams) (*anthropic.FederationRule, error) {
		return r.authTokenClient.Organization.Federation.Rules.Update(ctx, data.Id.ValueString(), *body)
	}, req, resp)
}

func (r *FederationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	fwresource.Delete(ctx, func(data *FederationRuleModel) error {
		_, err := r.authTokenClient.Organization.Federation.Rules.Archive(ctx, data.Id.ValueString())
		return err
	}, req, resp)
}

func (r *FederationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
