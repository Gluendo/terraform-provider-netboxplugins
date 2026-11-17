package crud

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/client"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/tfschema"
)

var (
	_ resource.Resource                = (*Resource)(nil)
	_ resource.ResourceWithConfigure   = (*Resource)(nil)
	_ resource.ResourceWithImportState = (*Resource)(nil)
)

// Resource serves one plugin model as a Terraform resource.
type Resource struct {
	codec
	srv *ServerData
}

// NewResource returns a constructor for the resource of m.
func NewResource(p *model.Plugin, m *model.Model) func() resource.Resource {
	return func() resource.Resource {
		return &Resource{codec: codec{plugin: p, model: m}}
	}
}

// Metadata implements resource.Resource.
func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.plugin.Name + "_" + r.model.Name
}

// Schema implements resource.Resource.
func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = tfschema.Resource(r.plugin, r.model)
}

// Configure implements resource.ResourceWithConfigure.
func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	srv, ok := req.ProviderData.(*ServerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.srv = srv
	resp.Diagnostics.Append(srv.CheckPlugin(r.plugin)...)
}

// Create implements resource.Resource.
func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan types.Object
	if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
		return
	}
	cft, err := r.customFieldTypes(ctx, r.srv)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read custom field definitions", err.Error())
		return
	}
	body, err := r.payload(ctx, plan.Attributes(), nil, cft)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	obj, err := r.srv.Client.Create(ctx, r.plugin.ListPath(r.model), body)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot create %s", r.typeName()), err.Error())
		return
	}
	r.setState(ctx, obj, plan.Attributes(), cft, &resp.State, &resp.Diagnostics)
}

// Read implements resource.Resource.
func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior types.Object
	if resp.Diagnostics.Append(req.State.Get(ctx, &prior)...); resp.Diagnostics.HasError() {
		return
	}
	id, ok := idOf(prior.Attributes())
	if !ok {
		resp.Diagnostics.AddError("Missing id in state", r.typeName())
		return
	}
	obj, err := r.srv.Client.Get(ctx, r.plugin.ObjectPath(r.model, id))
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot read %s %d", r.typeName(), id), err.Error())
		return
	}
	cft, err := r.customFieldTypes(ctx, r.srv)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read custom field definitions", err.Error())
		return
	}
	r.setState(ctx, obj, prior.Attributes(), cft, &resp.State, &resp.Diagnostics)
}

// Update implements resource.Resource.
func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, ok := idOf(prior.Attributes())
	if !ok {
		resp.Diagnostics.AddError("Missing id in state", r.typeName())
		return
	}
	cft, err := r.customFieldTypes(ctx, r.srv)
	if err != nil {
		resp.Diagnostics.AddError("Cannot read custom field definitions", err.Error())
		return
	}
	body, err := r.payload(ctx, plan.Attributes(), prior.Attributes(), cft)
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	objPath := r.plugin.ObjectPath(r.model, id)
	var obj map[string]any
	if len(body) == 0 {
		obj, err = r.srv.Client.Get(ctx, objPath)
	} else {
		obj, err = r.srv.Client.Patch(ctx, objPath, body)
	}
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot update %s %d", r.typeName(), id), err.Error())
		return
	}
	r.setState(ctx, obj, plan.Attributes(), cft, &resp.State, &resp.Diagnostics)
}

// Delete implements resource.Resource.
func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var prior types.Object
	if resp.Diagnostics.Append(req.State.Get(ctx, &prior)...); resp.Diagnostics.HasError() {
		return
	}
	id, ok := idOf(prior.Attributes())
	if !ok {
		return
	}
	err := r.srv.Client.Delete(ctx, r.plugin.ObjectPath(r.model, id))
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError(fmt.Sprintf("Cannot delete %s %d", r.typeName(), id), err.Error())
	}
}

// ImportState implements resource.ResourceWithImportState. The import ID is
// the NetBox ID of the object.
func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || id < 1 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected the numeric NetBox ID of the object, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.Int64Value(id))...)
}

type stateSetter interface {
	Set(ctx context.Context, val any) diag.Diagnostics
}

func (r *Resource) setState(ctx context.Context, obj map[string]any, prior map[string]attr.Value, cft map[string]string, st stateSetter, diags *diag.Diagnostics) {
	state, err := r.state(ctx, obj, prior, cft)
	if err != nil {
		diags.AddError(fmt.Sprintf("Cannot read %s response", r.typeName()), err.Error())
		return
	}
	diags.Append(st.Set(ctx, state)...)
}

func (r *Resource) typeName() string {
	return r.plugin.Name + "_" + r.model.Name
}

func idOf(attrs map[string]attr.Value) (int64, bool) {
	v, ok := attrs["id"].(types.Int64)
	if !ok || v.IsNull() || v.IsUnknown() {
		return 0, false
	}
	return v.ValueInt64(), true
}
