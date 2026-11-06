package provider

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/jianyuan/terraform-provider-anthropic/internal/apiclient"
	"github.com/jianyuan/terraform-provider-anthropic/internal/fwresource"
)

var _ resource.Resource = &FederationIssuerResource{}
var _ resource.ResourceWithConfigure = &FederationIssuerResource{}
var _ resource.ResourceWithImportState = &FederationIssuerResource{}

func NewFederationIssuerResource() resource.Resource {
	return &FederationIssuerResource{}
}

type FederationIssuerResource struct {
	baseResource
}

func (r *FederationIssuerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuer"
}

func (r *FederationIssuerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = federationIssuerSchema()
}

func (r *FederationIssuerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	fwresource.Create(ctx, func(_ *FederationIssuerModel, body *anthropic.OrganizationFederationIssuerNewParams) (*anthropic.FederationIssuer, error) {
		return r.authTokenClient.Organization.Federation.Issuers.New(ctx, *body)
	}, req, resp)
}

func (r *FederationIssuerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FederationIssuerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.authTokenClient.Organization.Federation.Issuers.Get(ctx, data.Id.ValueString())
	if err != nil {
		if apiclient.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Unable to read federation issuer", err.Error())
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

func (r *FederationIssuerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	fwresource.Update(ctx, func(data *FederationIssuerModel, body *anthropic.OrganizationFederationIssuerUpdateParams) (*anthropic.FederationIssuer, error) {
		return r.authTokenClient.Organization.Federation.Issuers.Update(ctx, data.Id.ValueString(), *body)
	}, req, resp)
}

func (r *FederationIssuerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	fwresource.Delete(ctx, func(data *FederationIssuerModel) error {
		_, err := r.authTokenClient.Organization.Federation.Issuers.Archive(ctx, data.Id.ValueString())
		return err
	}, req, resp)
}

func (r *FederationIssuerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
