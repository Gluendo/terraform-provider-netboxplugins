// Package tfvalue converts between plain field values (package shape) and
// Terraform attribute values.
package tfvalue

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// AttrType is the Terraform type of a field.
func AttrType(f *model.Field) attr.Type {
	switch f.Kind {
	case model.KindInt, model.KindRef:
		return types.Int64Type
	case model.KindFloat:
		return types.Float64Type
	case model.KindBool:
		return types.BoolType
	case model.KindRefSet:
		return types.SetType{ElemType: types.Int64Type}
	case model.KindIntList:
		if f.Set {
			return types.SetType{ElemType: types.Int64Type}
		}
		return types.ListType{ElemType: types.Int64Type}
	case model.KindStringList:
		if f.Set {
			return types.SetType{ElemType: types.StringType}
		}
		return types.ListType{ElemType: types.StringType}
	case model.KindTags:
		return types.SetType{ElemType: types.StringType}
	case model.KindCustomFields:
		return types.MapType{ElemType: types.StringType}
	default: // string, choice, json
		return types.StringType
	}
}

// ToTF converts a plain value to a Terraform value of the field's type.
// nil becomes null.
func ToTF(ctx context.Context, f *model.Field, v any) (attr.Value, error) {
	t := AttrType(f)
	if v == nil {
		return nullOf(t), nil
	}
	switch tt := t.(type) {
	case basetypes.StringType:
		s, ok := v.(string)
		if !ok {
			return nil, wrongType(f, v)
		}
		return types.StringValue(s), nil
	case basetypes.Int64Type:
		n, ok := v.(int64)
		if !ok {
			return nil, wrongType(f, v)
		}
		return types.Int64Value(n), nil
	case basetypes.Float64Type:
		n, ok := v.(float64)
		if !ok {
			return nil, wrongType(f, v)
		}
		return types.Float64Value(n), nil
	case basetypes.BoolType:
		b, ok := v.(bool)
		if !ok {
			return nil, wrongType(f, v)
		}
		return types.BoolValue(b), nil
	case basetypes.SetType:
		val, d := types.SetValueFrom(ctx, tt.ElemType, v)
		if d.HasError() {
			return nil, fmt.Errorf("field %q: %v", f.Name, d)
		}
		return val, nil
	case basetypes.ListType:
		val, d := types.ListValueFrom(ctx, tt.ElemType, v)
		if d.HasError() {
			return nil, fmt.Errorf("field %q: %v", f.Name, d)
		}
		return val, nil
	case basetypes.MapType:
		val, d := types.MapValueFrom(ctx, tt.ElemType, v)
		if d.HasError() {
			return nil, fmt.Errorf("field %q: %v", f.Name, d)
		}
		return val, nil
	}
	return nil, fmt.Errorf("field %q: unsupported type %T", f.Name, t)
}

// FromTF converts a known, non-null Terraform value to a plain value.
func FromTF(ctx context.Context, f *model.Field, v attr.Value) (any, error) {
	switch tv := v.(type) {
	case basetypes.StringValue:
		return tv.ValueString(), nil
	case basetypes.Int64Value:
		return tv.ValueInt64(), nil
	case basetypes.Float64Value:
		return tv.ValueFloat64(), nil
	case basetypes.BoolValue:
		return tv.ValueBool(), nil
	case basetypes.SetValue:
		return elements(ctx, f, tv.ElementType(ctx), tv.ElementsAs)
	case basetypes.ListValue:
		return elements(ctx, f, tv.ElementType(ctx), tv.ElementsAs)
	case basetypes.MapValue:
		out := map[string]string{}
		if d := tv.ElementsAs(ctx, &out, false); d.HasError() {
			return nil, fmt.Errorf("field %q: %v", f.Name, d)
		}
		return out, nil
	}
	return nil, fmt.Errorf("field %q: unsupported value %T", f.Name, v)
}

// elementsAs is the ElementsAs method of a set or list value.
type elementsAs func(ctx context.Context, target any, allowUnhandled bool) diag.Diagnostics

func elements(ctx context.Context, f *model.Field, et attr.Type, as elementsAs) (any, error) {
	var target any
	if _, ok := et.(basetypes.Int64Type); ok {
		out := []int64{}
		target = &out
		if d := as(ctx, target, false); d.HasError() {
			return nil, fmt.Errorf("field %q: %v", f.Name, d)
		}
		return out, nil
	}
	out := []string{}
	target = &out
	if d := as(ctx, target, false); d.HasError() {
		return nil, fmt.Errorf("field %q: %v", f.Name, d)
	}
	return out, nil
}

func nullOf(t attr.Type) attr.Value {
	switch tt := t.(type) {
	case basetypes.StringType:
		return types.StringNull()
	case basetypes.Int64Type:
		return types.Int64Null()
	case basetypes.Float64Type:
		return types.Float64Null()
	case basetypes.BoolType:
		return types.BoolNull()
	case basetypes.SetType:
		return types.SetNull(tt.ElemType)
	case basetypes.ListType:
		return types.ListNull(tt.ElemType)
	case basetypes.MapType:
		return types.MapNull(tt.ElemType)
	}
	return nil
}

func wrongType(f *model.Field, v any) error {
	return fmt.Errorf("field %q (%s): unexpected value type %T", f.Name, f.Kind, v)
}
