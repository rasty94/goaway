package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rasty94/terraform-provider-goaway/internal/client"
)

// --- goaway_upstream ---

type upstreamResource struct{ client *client.Client }

type upstreamModel struct {
	Address types.String `tfsdk:"address"`
}

func NewUpstreamResource() resource.Resource { return &upstreamResource{} }

func (r *upstreamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_upstream"
}

func (r *upstreamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an upstream DNS server in GoAway.",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{Required: true, Description: "Upstream address in host:port format (e.g. 1.1.1.1:53)."},
		},
	}
}

func (r *upstreamResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

func (r *upstreamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan upstreamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.AddUpstream(ctx, plan.Address.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to add upstream", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *upstreamResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

func (r *upstreamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan upstreamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_ = r.client.DeleteUpstream(ctx, state.Address.ValueString())
	if err := r.client.AddUpstream(ctx, plan.Address.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to update upstream", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *upstreamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state upstreamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUpstream(ctx, state.Address.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete upstream", err.Error())
	}
}

// --- goaway_resolution ---

type resolutionResource struct{ client *client.Client }

type resolutionModel struct {
	Domain types.String `tfsdk:"domain"`
	Value  types.String `tfsdk:"value"`
	Type   types.String `tfsdk:"type"`
}

func NewResolutionResource() resource.Resource { return &resolutionResource{} }

func (r *resolutionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resolution"
}

func (r *resolutionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a local DNS record (A, AAAA, or CNAME) in GoAway.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{Required: true},
			"value":  schema.StringAttribute{Required: true},
			"type":   schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("A")},
		},
	}
}

func (r *resolutionResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

func (r *resolutionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan resolutionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.CreateResolution(ctx, client.Resolution{
		Domain: plan.Domain.ValueString(),
		Value:  plan.Value.ValueString(),
		Type:   plan.Type.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create resolution", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resolutionResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

func (r *resolutionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan resolutionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_ = r.client.DeleteResolution(ctx, state.Domain.ValueString(), state.Value.ValueString())
	if err := r.client.CreateResolution(ctx, client.Resolution{
		Domain: plan.Domain.ValueString(),
		Value:  plan.Value.ValueString(),
		Type:   plan.Type.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to update resolution", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resolutionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state resolutionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteResolution(ctx, state.Domain.ValueString(), state.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete resolution", err.Error())
	}
}

// --- goaway_whitelist_domain ---

type whitelistResource struct{ client *client.Client }

type whitelistModel struct {
	Domain types.String `tfsdk:"domain"`
}

func NewWhitelistResource() resource.Resource { return &whitelistResource{} }

func (r *whitelistResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_whitelist_domain"
}

func (r *whitelistResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a whitelisted domain in GoAway.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{Required: true},
		},
	}
}

func (r *whitelistResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

func (r *whitelistResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan whitelistModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.AddWhitelist(ctx, plan.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to whitelist domain", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *whitelistResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

func (r *whitelistResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan whitelistModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_ = r.client.DeleteWhitelist(ctx, state.Domain.ValueString())
	if err := r.client.AddWhitelist(ctx, plan.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to update whitelist domain", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *whitelistResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state whitelistModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteWhitelist(ctx, state.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to remove whitelist domain", err.Error())
	}
}

// --- goaway_group ---

type groupResource struct{ client *client.Client }

type groupModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	UseGlobalPolicies types.Bool   `tfsdk:"use_global_policies"`
}

func NewGroupResource() resource.Resource { return &groupResource{} }

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a client policy group in GoAway.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true},
			"name":                schema.StringAttribute{Required: true},
			"description":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"use_global_policies": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateGroup(ctx, client.Group{
		Name:              plan.Name.ValueString(),
		Description:       plan.Description.ValueString(),
		UseGlobalPolicies: plan.UseGlobalPolicies.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create group", err.Error())
		return
	}
	plan.ID = types.StringValue(strconv.FormatUint(uint64(created.ID), 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := strconv.ParseUint(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid group ID", fmt.Sprintf("cannot parse %q: %v", state.ID.ValueString(), err))
		return
	}
	if err := r.client.DeleteGroup(ctx, uint(id)); err != nil {
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
	}
}
