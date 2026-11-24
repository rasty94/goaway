package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rasty94/terraform-provider-goaway/internal/client"
)

type goawayProvider struct {
	version string
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &goawayProvider{version: version}
	}
}

func (p *goawayProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "goaway"
	resp.Version = p.version
}

func (p *goawayProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage GoAway DNS sinkhole resources (upstreams, local DNS resolutions, whitelist domains, and client groups).",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the GoAway API (e.g. http://192.168.1.2:8080). Can also be set via GOAWAY_ENDPOINT.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "GoAway API key with write/admin scope. Can also be set via GOAWAY_API_KEY.",
			},
		},
	}
}

func (p *goawayProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("GOAWAY_ENDPOINT")
	if !cfg.Endpoint.IsNull() && cfg.Endpoint.ValueString() != "" {
		endpoint = cfg.Endpoint.ValueString()
	}
	apiKey := os.Getenv("GOAWAY_API_KEY")
	if !cfg.APIKey.IsNull() && cfg.APIKey.ValueString() != "" {
		apiKey = cfg.APIKey.ValueString()
	}

	if endpoint == "" {
		resp.Diagnostics.AddError("Missing GoAway Endpoint", "Set endpoint in the provider block or GOAWAY_ENDPOINT.")
		return
	}

	c := client.New(endpoint, apiKey)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *goawayProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUpstreamResource,
		NewResolutionResource,
		NewWhitelistResource,
		NewGroupResource,
	}
}

func (p *goawayProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
