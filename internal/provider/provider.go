package provider

import (
	"context"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	pframework "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type gorulesProvider struct{ version string }

type gorulesProviderModel struct {
	BaseURL types.String `tfsdk:"base_url"` // e.g. https://initial.gorules.io
	Token   types.String `tfsdk:"token"`    // PAT (Bearer)
}

type Config struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(version string) pframework.Provider {
	if version == "" {
		version = "dev"
	}
	return &gorulesProvider{version: version}
}

func (p *gorulesProvider) Metadata(_ context.Context, _ pframework.MetadataRequest, resp *pframework.MetadataResponse) {
	resp.TypeName = "gorules"
	resp.Version = p.version
}

func (p *gorulesProvider) Schema(_ context.Context, _ pframework.SchemaRequest, resp *pframework.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provider for GoRules BRMS.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Base URL of the BRMS (e.g. `https://initial.gorules.io`).",
			},
			"token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Personal Access Token (PAT) with appropriate permissions.",
			},
		},
	}
}

func (p *gorulesProvider) Configure(ctx context.Context, req pframework.ConfigureRequest, resp *pframework.ConfigureResponse) {
	var data gorulesProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg := &Config{
		BaseURL: strings.TrimRight(data.BaseURL.ValueString(), "/"),
		Token:   data.Token.ValueString(),
		HTTP:    &http.Client{},
	}
	if cfg.BaseURL == "" || cfg.Token == "" {
		resp.Diagnostics.AddError("invalid config", "base_url and token are required")
		return
	}
	resp.DataSourceData = cfg
	resp.ResourceData = cfg
}

func (p *gorulesProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,     // project resource
		NewEnvironmentResource, // environment resource
		NewGroupResource,       // group resource
	}
}

func (p *gorulesProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }
