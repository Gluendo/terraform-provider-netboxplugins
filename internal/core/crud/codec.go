package crud

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/shape"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/tfschema"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/tfvalue"
)

// codec converts between Terraform objects and NetBox API objects for one
// model.
type codec struct {
	plugin     *model.Plugin
	model      *model.Model
	dataSource bool
}

func (c *codec) attrTypes() map[string]attr.Type {
	out := map[string]attr.Type{"id": types.Int64Type}
	if c.dataSource {
		out[tfschema.FiltersAttr] = types.MapType{ElemType: types.StringType}
	}
	for i := range c.model.Fields {
		f := &c.model.Fields[i]
		if c.dataSource && f.WriteOnly {
			continue
		}
		out[f.Name] = tfvalue.AttrType(f)
	}
	return out
}

// customFieldTypes returns custom field definitions when the model has
// custom fields.
func (c *codec) customFieldTypes(ctx context.Context, srv *ServerData) (shape.CustomFieldTypes, error) {
	if !c.model.HasKind(model.KindCustomFields) {
		return nil, nil
	}
	return srv.CustomFieldTypes(ctx)
}

// payload builds the API body from planned values. With prior set, only
// attributes whose planned value differs from prior are included (PATCH,
// ADR-0006). Unknown and null values are never sent.
func (c *codec) payload(ctx context.Context, plan, prior map[string]attr.Value, cft shape.CustomFieldTypes) (map[string]any, error) {
	out := map[string]any{}
	for i := range c.model.Fields {
		f := &c.model.Fields[i]
		if f.ReadOnly {
			continue
		}
		pv, ok := plan[f.Name]
		if !ok || pv.IsNull() || pv.IsUnknown() {
			continue
		}
		if prior != nil {
			if sv, ok := prior[f.Name]; ok && pv.Equal(sv) {
				continue
			}
		}
		plain, err := tfvalue.FromTF(ctx, f, pv)
		if err != nil {
			return nil, err
		}
		api, err := shape.ToAPI(f, plain, cft)
		if err != nil {
			return nil, err
		}
		out[f.API] = api
	}
	return out, nil
}

// state builds the Terraform object from an API object. prior supplies
// values NetBox does not return (write-only fields, data source filters) and
// the custom field keys under management.
func (c *codec) state(ctx context.Context, obj map[string]any, prior map[string]attr.Value, cft shape.CustomFieldTypes) (types.Object, error) {
	values := map[string]attr.Value{}
	id, err := objectID(obj)
	if err != nil {
		return types.Object{}, err
	}
	values["id"] = types.Int64Value(id)
	if c.dataSource {
		values[tfschema.FiltersAttr] = types.MapNull(types.StringType)
		if pv, ok := prior[tfschema.FiltersAttr]; ok {
			values[tfschema.FiltersAttr] = pv
		}
	}
	for i := range c.model.Fields {
		f := &c.model.Fields[i]
		if f.WriteOnly {
			if c.dataSource {
				continue
			}
			v, err := tfvalue.ToTF(ctx, f, nil)
			if err != nil {
				return types.Object{}, err
			}
			if pv, ok := prior[f.Name]; ok && !pv.IsUnknown() {
				v = pv
			}
			values[f.Name] = v
			continue
		}
		plain, err := shape.FromAPI(f, obj[f.API], cft)
		if err != nil {
			return types.Object{}, fmt.Errorf("%s: %w", c.model.Name, err)
		}
		if f.Kind == model.KindCustomFields {
			plain = keepManagedKeys(plain.(map[string]string), prior[f.Name])
		}
		v, err := tfvalue.ToTF(ctx, f, plain)
		if err != nil {
			return types.Object{}, err
		}
		values[f.Name] = v
	}
	obj2, d := types.ObjectValue(c.attrTypes(), values)
	if d.HasError() {
		return types.Object{}, fmt.Errorf("%s: build state: %v", c.model.Name, d)
	}
	return obj2, nil
}

// keepManagedKeys restricts custom fields to the keys of prior when prior is
// a known map (ADR-0006). Without a known prior every non-null value is kept.
func keepManagedKeys(all map[string]string, prior attr.Value) map[string]string {
	pm, ok := prior.(types.Map)
	if !ok || pm.IsNull() || pm.IsUnknown() {
		return all
	}
	out := map[string]string{}
	for k := range pm.Elements() {
		if v, ok := all[k]; ok {
			out[k] = v
		} else {
			// Managed key now empty in NetBox: report the drift as "".
			out[k] = ""
		}
	}
	return out
}

func objectID(obj map[string]any) (int64, error) {
	f := &model.Field{Name: "id", Kind: model.KindInt}
	v, err := shape.FromAPI(f, obj["id"], nil)
	if err != nil {
		return 0, fmt.Errorf("API object id: %w", err)
	}
	if v == nil {
		return 0, fmt.Errorf("API object without id")
	}
	return v.(int64), nil
}
