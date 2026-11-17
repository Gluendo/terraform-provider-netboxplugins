// Package provider is the netboxplugins Terraform provider.
package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/client"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/crud"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/detect"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// TypeName is the provider type and the prefix of every resource.
const TypeName = "netboxplugins"

var _ provider.Provider = (*Provider)(nil)

// Provider serves every plugin folder compiled into the binary.
type Provider struct {
	version string
	plugins []*model.Plugin
}

// New returns a provider constructor serving plugins.
func New(version string, plugins []*model.Plugin) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version, plugins: plugins}
	}
}

// config mirrors the provider block. Names and environment variables match
// the core NetBox provider (ADR-0004).
type config struct {
	ServerURL          types.String `tfsdk:"server_url"`
	APIToken           types.String `tfsdk:"api_token"`
	AllowInsecureHTTPS types.Bool   `tfsdk:"allow_insecure_https"`
	RequestTimeout     types.Int64  `tfsdk:"request_timeout"`
	Headers            types.Map    `tfsdk:"headers"`
	SkipVersionCheck   types.Bool   `tfsdk:"skip_version_check"`
}

// Metadata implements provider.Provider.
func (p *Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = TypeName
	resp.Version = p.version
}

// Schema implements provider.Provider.
func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages objects of NetBox plugins. Use it next to the `e-breuninger/netbox` provider, " +
			"which manages core NetBox objects; both read the same environment variables.",
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "NetBox URL, for example `https://netbox.example.com`. Defaults to `NETBOX_SERVER_URL`.",
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "NetBox API token: the full `nbt_<key>.<secret>` value for a v2 token, or a v1 token. Defaults to `NETBOX_API_TOKEN`.",
			},
			"allow_insecure_https": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip TLS certificate verification. Defaults to `NETBOX_ALLOW_INSECURE_HTTPS`, else `false`.",
			},
			"request_timeout": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "HTTP request timeout in seconds. Defaults to `NETBOX_REQUEST_TIMEOUT`, else 30.",
			},
			"headers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "Extra HTTP headers sent with every request.",
			},
			"skip_version_check": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Do not warn when NetBox or a plugin runs a version outside the tested combinations. Defaults to `NETBOX_SKIP_VERSION_CHECK`, else `false`.",
			},
		},
	}
}

// Configure implements provider.Provider.
func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg config
	if resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...); resp.Diagnostics.HasError() {
		return
	}
	if cfg.ServerURL.IsUnknown() || cfg.APIToken.IsUnknown() || cfg.Headers.IsUnknown() {
		// Values come from resources not created yet; resources stay unconfigured until then.
		return
	}

	serverURL := stringOr(cfg.ServerURL, os.Getenv("NETBOX_SERVER_URL"))
	token := stringOr(cfg.APIToken, os.Getenv("NETBOX_API_TOKEN"))
	if serverURL == "" {
		resp.Diagnostics.AddError("Missing NetBox URL", "Set server_url or NETBOX_SERVER_URL.")
	}
	if token == "" {
		resp.Diagnostics.AddError("Missing NetBox API token", "Set api_token or NETBOX_API_TOKEN.")
	}
	insecure, err := boolOr(cfg.AllowInsecureHTTPS, "NETBOX_ALLOW_INSECURE_HTTPS")
	if err != nil {
		resp.Diagnostics.AddError("Invalid allow_insecure_https", err.Error())
	}
	skip, err := boolOr(cfg.SkipVersionCheck, "NETBOX_SKIP_VERSION_CHECK")
	if err != nil {
		resp.Diagnostics.AddError("Invalid skip_version_check", err.Error())
	}
	timeout := int64(30)
	switch {
	case !cfg.RequestTimeout.IsNull() && !cfg.RequestTimeout.IsUnknown():
		timeout = cfg.RequestTimeout.ValueInt64()
	case os.Getenv("NETBOX_REQUEST_TIMEOUT") != "":
		if timeout, err = strconv.ParseInt(os.Getenv("NETBOX_REQUEST_TIMEOUT"), 10, 64); err != nil {
			resp.Diagnostics.AddError("Invalid NETBOX_REQUEST_TIMEOUT", err.Error())
		}
	}
	headers := map[string]string{}
	if !cfg.Headers.IsNull() {
		resp.Diagnostics.Append(cfg.Headers.ElementsAs(ctx, &headers, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(client.Config{
		ServerURL:          serverURL,
		APIToken:           token,
		AllowInsecureHTTPS: insecure,
		RequestTimeout:     time.Duration(timeout) * time.Second,
		Headers:            headers,
		UserAgent:          fmt.Sprintf("terraform-provider-%s/%s", TypeName, p.version),
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	info, err := detect.Fetch(ctx, c)
	if err != nil {
		resp.Diagnostics.AddError("Cannot reach NetBox", err.Error())
		return
	}
	srv := &crud.ServerData{Client: c, Info: info, SkipVersionCheck: skip}
	resp.ResourceData = srv
	resp.DataSourceData = srv
}

// Resources implements provider.Provider.
func (p *Provider) Resources(_ context.Context) []func() resource.Resource {
	var out []func() resource.Resource
	for _, pl := range p.plugins {
		for i := range pl.Models {
			out = append(out, crud.NewResource(pl, &pl.Models[i]))
		}
	}
	return out
}

// DataSources implements provider.Provider.
func (p *Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	var out []func() datasource.DataSource
	for _, pl := range p.plugins {
		for i := range pl.Models {
			out = append(out, crud.NewDataSource(pl, &pl.Models[i]))
		}
	}
	return out
}

func stringOr(v types.String, fallback string) string {
	if v.IsNull() || v.IsUnknown() {
		return fallback
	}
	return v.ValueString()
}

func boolOr(v types.Bool, env string) (bool, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool(), nil
	}
	if s := os.Getenv(env); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return false, fmt.Errorf("%s: %w", env, err)
		}
		return b, nil
	}
	return false, nil
}
