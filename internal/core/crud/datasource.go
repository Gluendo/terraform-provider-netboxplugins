package crud

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/tfschema"
)

var (
	_ datasource.DataSource              = (*DataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*DataSource)(nil)
)

// DataSource serves one plugin model as a Terraform data source.
type DataSource struct {
	codec
	srv *ServerData
}

// NewDataSource returns a constructor for the data source of m.
func NewDataSource(p *model.Plugin, m *model.Model) func() datasource.DataSource {
	return func() datasource.DataSource {
		return &DataSource{codec: codec{plugin: p, model: m, dataSource: true}}
	}
}

// Metadata implements datasource.DataSource.
func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.plugin.Name + "_" + d.model.Name
}

// Schema implements datasource.DataSource.
func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = tfschema.DataSource(d.plugin, d.model)
}

// Configure implements datasource.DataSourceWithConfigure.
func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	srv, ok := req.ProviderData.(*ServerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	d.srv = srv
	resp.Diagnostics.Append(srv.CheckPlugin(d.plugin)...)
}

// Read implements datasource.DataSource.
func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg types.Object
	if resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...); resp.Diagnostics.HasError() {
		return
	}
	attrs := cfg.Attributes()
	name := d.plugin.Name + "_" + d.model.Name

	var obj map[string]any
	var err error
	filters, _ := attrs[tfschema.FiltersAttr].(types.Map)
	switch id, hasID := idOf(attrs); {
	case hasID:
		obj, err = d.srv.Client.Get(ctx, d.plugin.ObjectPath(d.model, id))
	case !filters.IsNull() && !filters.IsUnknown():
		q := url.Values{}
		for k, v := range filters.Elements() {
			s, ok := v.(types.String)
			if !ok || s.IsNull() || s.IsUnknown() {
				continue
			}
			q.Add(k, s.ValueString())
		}
		var items []map[string]any
		items, err = d.srv.Client.List(ctx, d.plugin.ListPath(d.model), q, 2)
		if err == nil && len(items) != 1 {
			err = fmt.Errorf("filters %v matched %s; they must match exactly one object", q, count(len(items)))
		}
		if err == nil {
			obj = items[0]
		}
	default:
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot read %s", name), "Set id or filters.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot read %s", name), err.Error())
		return
	}
	cft, err := d.customFieldTypes(ctx, d.srv)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read custom field definitions", err.Error())
		return
	}
	state, err := d.state(ctx, obj, attrs, cft)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot read %s response", name), err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func count(n int) string {
	switch n {
	case 0:
		return "no object"
	case 1:
		return "one object"
	}
	return "more than one object"
}
