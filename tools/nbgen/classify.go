package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// Fields every NetBox model returns that carry no information for Terraform.
var noiseFields = map[string]bool{
	"id": true, "url": true, "display": true, "display_url": true,
	"created": true, "last_updated": true,
}

// Attribute names Terraform reserves in resource and data source blocks.
var reservedNames = map[string]bool{
	"count": true, "depends_on": true, "for_each": true, "lifecycle": true,
	"provider": true, "provisioner": true, "connection": true,
}

// schemaDoc is the parts of an OpenAPI document nbgen reads.
type schemaDoc struct {
	Paths      map[string]map[string]any `json:"paths"`
	Components struct {
		Schemas map[string]map[string]any `json:"schemas"`
	} `json:"components"`
}

// node is a resolved schema: references and single-element allOf removed,
// nullability carried over from the wrappers.
type node struct {
	schema   map[string]any
	nullable bool
	refName  string // component name of the last $ref followed
}

func (d *schemaDoc) resolve(s map[string]any) node {
	n := node{schema: s}
	for i := 0; i < 32 && n.schema != nil; i++ {
		if b, _ := n.schema["nullable"].(bool); b {
			n.nullable = true
		}
		if ref, ok := n.schema["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			n.refName = name
			n.schema = d.Components.Schemas[name]
			continue
		}
		if all, ok := n.schema["allOf"].([]any); ok && len(all) == 1 {
			n.schema, _ = all[0].(map[string]any)
			continue
		}
		break
	}
	return n
}

func (n node) typ() string { s, _ := n.schema["type"].(string); return s }

func (n node) enum() []string {
	vals, _ := n.schema["enum"].([]any)
	var out []string
	for _, v := range vals {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (n node) props() map[string]any { p, _ := n.schema["properties"].(map[string]any); return p }

// isObjectRef reports whether n is a nested object identified by an id.
func (n node) isObjectRef() bool {
	_, ok := n.props()["id"]
	return ok
}

// isChoiceObject reports whether n is NetBox's {value, label} choice object.
func (n node) isChoiceObject() bool {
	p := n.props()
	_, v := p["value"]
	_, l := p["label"]
	return v && l
}

// oneOfInt reports whether n is a oneOf whose branches include an integer,
// NetBox's way of accepting either an ID or a nested object.
func (d *schemaDoc) oneOfInt(n node) bool {
	branches, _ := n.schema["oneOf"].([]any)
	for _, b := range branches {
		bm, _ := b.(map[string]any)
		if d.resolve(bm).typ() == "integer" {
			return true
		}
	}
	return false
}

// oneOfString reports whether every branch of a oneOf is a string.
func (d *schemaDoc) oneOfString(n node) bool {
	branches, _ := n.schema["oneOf"].([]any)
	if len(branches) == 0 {
		return false
	}
	for _, b := range branches {
		bm, _ := b.(map[string]any)
		if d.resolve(bm).typ() != "string" {
			return false
		}
	}
	return true
}

func (d *schemaDoc) items(n node) node {
	it, _ := n.schema["items"].(map[string]any)
	return d.resolve(it)
}

// classifyWritable assigns a kind to a field present in the write schema,
// comparing it with the read schema (ADR-0002).
func (d *schemaDoc) classifyWritable(name string, w, r node, hasRead bool) (model.Kind, []string, error) {
	switch name {
	case "tags":
		return model.KindTags, nil, nil
	case "custom_fields":
		return model.KindCustomFields, nil, nil
	}
	readRef := hasRead && r.isObjectRef()
	switch {
	case d.oneOfInt(w):
		return model.KindRef, nil, nil
	case (w.typ() == "integer" || w.typ() == "object" || w.refName != "") && readRef:
		return model.KindRef, nil, nil
	case w.typ() == "array":
		wi := d.items(w)
		var ri node
		if hasRead {
			ri = d.items(r)
		}
		// Read items are nested objects with an id, written either as IDs or
		// as nested request objects (which carry no id).
		readRefItems := hasRead && ri.isObjectRef()
		refItems := d.oneOfInt(wi) || wi.isObjectRef() ||
			(readRefItems && (wi.typ() == "integer" || wi.typ() == "object" || wi.refName != ""))
		switch {
		case refItems:
			return model.KindRefSet, nil, nil
		case wi.typ() == "integer":
			return model.KindIntList, nil, nil
		case wi.typ() == "string" || d.oneOfString(wi):
			return model.KindStringList, wi.enum(), nil
		}
		return model.KindJSON, nil, nil
	case w.typ() == "string" && len(w.enum()) > 0:
		if hasRead && r.isChoiceObject() {
			return model.KindChoice, w.enum(), nil
		}
		return model.KindString, w.enum(), nil
	case w.typ() == "string" || d.oneOfString(w):
		return model.KindString, nil, nil
	case w.typ() == "integer":
		return model.KindInt, nil, nil
	case w.typ() == "number":
		return model.KindFloat, nil, nil
	case w.typ() == "boolean":
		return model.KindBool, nil, nil
	case w.typ() == "object" || w.typ() == "":
		return model.KindJSON, nil, nil
	}
	return "", nil, fmt.Errorf("field %q: cannot classify write type %q", name, w.typ())
}

// classifyReadOnly assigns a kind to a field only present in the read schema.
// It returns ok=false for fields not worth exporting.
func (d *schemaDoc) classifyReadOnly(name string, r node) (model.Kind, bool) {
	// Counters and reverse-relation lists change whenever other objects
	// change; they are not exported (ADR-0002).
	if noiseFields[name] || strings.HasPrefix(name, "_") || strings.HasSuffix(name, "_count") {
		return "", false
	}
	switch {
	case r.isChoiceObject():
		return model.KindChoice, true
	case r.isObjectRef():
		return model.KindRef, true
	case r.typ() == "array":
		ri := d.items(r)
		switch {
		case ri.refName == "NestedTag":
			return model.KindTags, true
		case ri.isObjectRef():
			return "", false
		case ri.typ() == "integer":
			return model.KindIntList, true
		case ri.typ() == "string":
			return model.KindStringList, true
		}
		return "", false
	case r.typ() == "string" || d.oneOfString(r):
		return model.KindString, true
	case r.typ() == "integer":
		return model.KindInt, true
	case r.typ() == "number":
		return model.KindFloat, true
	case r.typ() == "boolean":
		return model.KindBool, true
	}
	return "", false
}

// operationSchemas finds the write and read schemas of an endpoint from its
// operations: the POST request body and the detail GET response.
func (d *schemaDoc) operationSchemas(listPath string) (write, read node, err error) {
	list, ok := d.Paths[listPath]
	if !ok {
		return node{}, node{}, fmt.Errorf("path %s not in schema", listPath)
	}
	post, ok := list["post"].(map[string]any)
	if !ok {
		return node{}, node{}, fmt.Errorf("path %s has no POST operation", listPath)
	}
	body := dig(post, "requestBody", "content", "application/json", "schema")
	if one, ok := body["oneOf"].([]any); ok && len(one) > 0 {
		body, _ = one[0].(map[string]any)
	}
	write = d.resolve(body)

	detail, ok := d.Paths[listPath+"{id}/"]
	if !ok {
		return node{}, node{}, fmt.Errorf("path %s{id}/ not in schema", listPath)
	}
	get, _ := detail["get"].(map[string]any)
	read = d.resolve(dig(get, "responses", "200", "content", "application/json", "schema"))
	if write.schema == nil || read.schema == nil {
		return node{}, node{}, fmt.Errorf("path %s: cannot resolve request or response schema", listPath)
	}
	return write, read, nil
}

func dig(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		next, _ := m[k].(map[string]any)
		m = next
	}
	return m
}

func requiredSet(n node) map[string]bool {
	out := map[string]bool{}
	req, _ := n.schema["required"].([]any)
	for _, r := range req {
		if s, ok := r.(string); ok {
			out[s] = true
		}
	}
	return out
}

// propertyOrder returns write properties first, then read-only ones, each
// group sorted, so generated output is stable.
func propertyOrder(write, read node) (writable, readOnly []string) {
	for n := range write.props() {
		writable = append(writable, n)
	}
	for n := range read.props() {
		if _, ok := write.props()[n]; !ok {
			readOnly = append(readOnly, n)
		}
	}
	sort.Strings(writable)
	sort.Strings(readOnly)
	return writable, readOnly
}

// description returns the description written on a property itself, without
// drf-spectacular's generated choice lists. Descriptions of referenced
// components are serializer docstrings and are ignored.
func description(props ...map[string]any) string {
	for _, p := range props {
		if p == nil {
			continue
		}
		d, _ := p["description"].(string)
		var keep []string
		for _, line := range strings.Split(d, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "* `") {
				continue
			}
			keep = append(keep, line)
		}
		if s := strings.TrimSpace(strings.Join(keep, " ")); s != "" {
			return s
		}
	}
	return ""
}
