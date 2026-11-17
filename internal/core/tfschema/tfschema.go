// Package tfschema builds Terraform schemas from model descriptors.
package tfschema

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/tfvalue"
)

// FiltersAttr is the data source attribute holding NetBox query filters.
const FiltersAttr = "filters"

const managedNote = "Only attributes set in the configuration are managed: an optional attribute left out takes the value NetBox holds and is never sent. To clear a value, set it explicitly (`\"\"`, `[]`). For `custom_fields`, only the keys set in the configuration are managed."

// Resource builds the resource schema of a model (ADR-0006).
func Resource(p *model.Plugin, m *model.Model) rschema.Schema {
	attrs := map[string]rschema.Attribute{
		"id": rschema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "NetBox ID of the object.",
			PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		},
	}
	for i := range m.Fields {
		f := &m.Fields[i]
		attrs[f.Name] = resourceAttr(f)
	}
	return rschema.Schema{
		MarkdownDescription: fmt.Sprintf("%s\n\nPlugin: `%s` (%s). API: `%s`.\n\n%s",
			m.Description, p.Package, p.Maturity, p.ListPath(m), managedNote),
		Attributes: attrs,
	}
}

// DataSource builds the data source schema of a model: every readable field
// computed, looked up by id or by filters.
func DataSource(p *model.Plugin, m *model.Model) dschema.Schema {
	attrs := map[string]dschema.Attribute{
		"id": dschema.Int64Attribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "NetBox ID of the object. Set either `id` or `filters`.",
		},
		FiltersAttr: dschema.MapAttribute{
			ElementType: types.StringType,
			Optional:    true,
			MarkdownDescription: fmt.Sprintf("NetBox query parameters on `%s` that select exactly one object, "+
				"for example `{ name = \"example\" }`.", p.ListPath(m)),
		},
	}
	for i := range m.Fields {
		f := &m.Fields[i]
		if f.WriteOnly {
			continue
		}
		attrs[f.Name] = dataSourceAttr(f)
	}
	return dschema.Schema{
		MarkdownDescription: fmt.Sprintf("Reads one %s.\n\nPlugin: `%s` (%s). API: `%s`.",
			strings.TrimSuffix(m.Description, "."), p.Package, p.Maturity, p.ListPath(m)),
		Attributes: attrs,
	}
}

type flags struct {
	required, optional, computed, keepState bool
}

func resourceFlags(f *model.Field) flags {
	switch {
	case f.ReadOnly:
		return flags{computed: true}
	case f.Required:
		return flags{required: true}
	case f.WriteOnly:
		return flags{optional: true}
	default:
		return flags{optional: true, computed: true, keepState: true}
	}
}

func fieldDescription(f *model.Field) string {
	parts := []string{}
	if f.Description != "" {
		parts = append(parts, strings.TrimSuffix(f.Description, ".")+".")
	}
	switch f.Kind {
	case model.KindRef:
		parts = append(parts, "NetBox ID of the referenced object.")
	case model.KindRefSet:
		parts = append(parts, "NetBox IDs of the referenced objects.")
	case model.KindTags:
		parts = append(parts, "Tag names.")
	case model.KindCustomFields:
		parts = append(parts, "Custom field values as strings; lists and objects as JSON (`jsonencode`).")
	case model.KindJSON:
		parts = append(parts, "JSON value (`jsonencode`).")
	}
	if len(f.Choices) > 0 {
		q := make([]string, len(f.Choices))
		for i, c := range f.Choices {
			q[i] = "`" + c + "`"
		}
		parts = append(parts, "One of "+strings.Join(q, ", ")+".")
	}
	if f.WriteOnly {
		parts = append(parts, "Sent on create and update; NetBox does not return it.")
	}
	return strings.Join(parts, " ")
}

//nolint:gocyclo // one case per attribute type
func resourceAttr(f *model.Field) rschema.Attribute {
	fl := resourceFlags(f)
	desc := fieldDescription(f)
	switch t := tfvalue.AttrType(f).(type) {
	case basetypes.StringType:
		a := rschema.StringAttribute{Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if len(f.Choices) > 0 && !f.ReadOnly {
			a.Validators = []validator.String{stringvalidator.OneOf(f.Choices...)}
		}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.Int64Type:
		a := rschema.Int64Attribute{Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if f.Kind == model.KindRef && !f.ReadOnly {
			a.Validators = []validator.Int64{int64validator.AtLeast(1)}
		}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.Float64Type:
		a := rschema.Float64Attribute{Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.Float64{float64planmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.BoolType:
		a := rschema.BoolAttribute{Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.SetType:
		a := rschema.SetAttribute{ElementType: t.ElemType, Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.Set{setplanmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.ListType:
		a := rschema.ListAttribute{ElementType: t.ElemType, Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.List{listplanmodifier.UseStateForUnknown()}
		}
		return a
	case basetypes.MapType:
		a := rschema.MapAttribute{ElementType: t.ElemType, Required: fl.required, Optional: fl.optional, Computed: fl.computed, Sensitive: f.Sensitive, MarkdownDescription: desc}
		if fl.keepState {
			a.PlanModifiers = []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}
		}
		return a
	}
	panic(fmt.Sprintf("field %q: no schema for kind %q", f.Name, f.Kind))
}

func dataSourceAttr(f *model.Field) dschema.Attribute {
	desc := fieldDescription(f)
	switch t := tfvalue.AttrType(f).(type) {
	case basetypes.StringType:
		return dschema.StringAttribute{Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.Int64Type:
		return dschema.Int64Attribute{Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.Float64Type:
		return dschema.Float64Attribute{Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.BoolType:
		return dschema.BoolAttribute{Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.SetType:
		return dschema.SetAttribute{ElementType: t.ElemType, Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.ListType:
		return dschema.ListAttribute{ElementType: t.ElemType, Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	case basetypes.MapType:
		return dschema.MapAttribute{ElementType: t.ElemType, Computed: true, Sensitive: f.Sensitive, MarkdownDescription: desc}
	}
	panic(fmt.Sprintf("field %q: no schema for kind %q", f.Name, f.Kind))
}
