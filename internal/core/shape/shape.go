// Package shape converts field values between NetBox's API formats and plain
// Go values (ADR-0002).
//
// Plain values are nil, string, int64, float64, bool, []int64, []string and
// map[string]string. API values are what encoding/json produces with
// UseNumber: nil, string, json.Number, bool, []any and map[string]any.
package shape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// CustomFieldTypes maps a custom field name to its NetBox type
// (text, longtext, integer, decimal, boolean, date, datetime, url, json,
// select, multiselect, object, multiobject).
type CustomFieldTypes map[string]string

// Decode parses an API response body, keeping numbers exact.
func Decode(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// FromAPI converts the read format of a field to its plain value.
func FromAPI(f *model.Field, raw any, cft CustomFieldTypes) (any, error) {
	switch f.Kind {
	case model.KindString:
		if raw == nil {
			return nil, nil
		}
		return scalarString(raw)
	case model.KindInt:
		if raw == nil {
			return nil, nil
		}
		return toInt64(raw)
	case model.KindFloat:
		if raw == nil {
			return nil, nil
		}
		return toFloat64(raw)
	case model.KindBool:
		if raw == nil {
			return nil, nil
		}
		b, ok := raw.(bool)
		if !ok {
			return nil, typeErr(f, "boolean", raw)
		}
		return b, nil
	case model.KindChoice:
		return choiceValue(raw)
	case model.KindRef:
		return refID(raw)
	case model.KindRefSet:
		items, err := list(f, raw)
		if err != nil || items == nil {
			return []int64{}, err
		}
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			id, err := refID(it)
			if err != nil {
				return nil, err
			}
			if id != nil {
				ids = append(ids, id.(int64))
			}
		}
		slices.Sort(ids)
		return ids, nil
	case model.KindIntList:
		items, err := list(f, raw)
		if err != nil || items == nil {
			return nil, err
		}
		out := make([]int64, 0, len(items))
		for _, it := range items {
			n, err := toInt64(it)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		if f.Set {
			slices.Sort(out)
		}
		return out, nil
	case model.KindStringList:
		items, err := list(f, raw)
		if err != nil || items == nil {
			return nil, err
		}
		out := make([]string, 0, len(items))
		for _, it := range items {
			s, err := scalarString(it)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		if f.Set {
			slices.Sort(out)
		}
		return out, nil
	case model.KindTags:
		items, err := list(f, raw)
		if err != nil || items == nil {
			return []string{}, err
		}
		names := make([]string, 0, len(items))
		for _, it := range items {
			switch t := it.(type) {
			case map[string]any:
				name, err := scalarString(t["name"])
				if err != nil {
					return nil, err
				}
				names = append(names, name)
			case string:
				names = append(names, t)
			default:
				return nil, typeErr(f, "tag object", it)
			}
		}
		slices.Sort(names)
		return names, nil
	case model.KindCustomFields:
		if raw == nil {
			return map[string]string{}, nil
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, typeErr(f, "object", raw)
		}
		out := make(map[string]string, len(m))
		for name, v := range m {
			if v == nil {
				continue
			}
			s, err := customFieldFromAPI(cft[name], v)
			if err != nil {
				return nil, fmt.Errorf("custom field %q: %w", name, err)
			}
			out[name] = s
		}
		return out, nil
	case model.KindJSON:
		if raw == nil {
			return nil, nil
		}
		return canonicalJSON(raw)
	}
	return nil, fmt.Errorf("field %q: unknown kind %q", f.Name, f.Kind)
}

// ToAPI converts a plain value to the write format of a field.
func ToAPI(f *model.Field, v any, cft CustomFieldTypes) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch f.Kind {
	case model.KindString, model.KindChoice, model.KindInt, model.KindFloat, model.KindBool,
		model.KindRef, model.KindRefSet, model.KindIntList, model.KindStringList:
		return v, nil
	case model.KindTags:
		names, ok := v.([]string)
		if !ok {
			return nil, typeErr(f, "[]string", v)
		}
		out := make([]map[string]string, 0, len(names))
		for _, n := range names {
			out = append(out, map[string]string{"name": n})
		}
		return out, nil
	case model.KindCustomFields:
		m, ok := v.(map[string]string)
		if !ok {
			return nil, typeErr(f, "map[string]string", v)
		}
		out := make(map[string]any, len(m))
		for name, s := range m {
			val, err := customFieldToAPI(cft[name], s)
			if err != nil {
				return nil, fmt.Errorf("custom field %q: %w", name, err)
			}
			out[name] = val
		}
		return out, nil
	case model.KindJSON:
		s, ok := v.(string)
		if !ok {
			return nil, typeErr(f, "JSON string", v)
		}
		return Decode([]byte(s))
	}
	return nil, fmt.Errorf("field %q: unknown kind %q", f.Name, f.Kind)
}

func customFieldFromAPI(cfType string, v any) (string, error) {
	switch cfType {
	case "select":
		s, err := choiceValue(v)
		if err != nil || s == nil {
			return "", err
		}
		return s.(string), nil
	case "multiselect":
		items, ok := v.([]any)
		if !ok {
			return "", fmt.Errorf("expected a list, got %T", v)
		}
		vals := make([]string, 0, len(items))
		for _, it := range items {
			s, err := choiceValue(it)
			if err != nil {
				return "", err
			}
			if s != nil {
				vals = append(vals, s.(string))
			}
		}
		return marshal(vals)
	case "object":
		id, err := refID(v)
		if err != nil || id == nil {
			return "", err
		}
		return strconv.FormatInt(id.(int64), 10), nil
	case "multiobject":
		items, ok := v.([]any)
		if !ok {
			return "", fmt.Errorf("expected a list, got %T", v)
		}
		ids := make([]int64, 0, len(items))
		for _, it := range items {
			id, err := refID(it)
			if err != nil {
				return "", err
			}
			if id != nil {
				ids = append(ids, id.(int64))
			}
		}
		slices.Sort(ids)
		return marshal(ids)
	case "json":
		return canonicalJSON(v)
	}
	// Scalar types, and unknown types read without definitions.
	switch t := v.(type) {
	case map[string]any:
		if val, ok := t["value"]; ok {
			s, err := choiceValue(val)
			if err != nil || s == nil {
				return "", err
			}
			return s.(string), nil
		}
		if id, ok := t["id"]; ok {
			n, err := toInt64(id)
			return strconv.FormatInt(n, 10), err
		}
		return canonicalJSON(t)
	case []any:
		return canonicalJSON(t)
	}
	return scalarString(v)
}

func customFieldToAPI(cfType, s string) (any, error) {
	switch cfType {
	case "integer":
		if s == "" {
			return nil, nil
		}
		return strconv.ParseInt(s, 10, 64)
	case "decimal":
		if s == "" {
			return nil, nil
		}
		return strconv.ParseFloat(s, 64)
	case "boolean":
		if s == "" {
			return nil, nil
		}
		return strconv.ParseBool(s)
	case "select", "date", "datetime", "url":
		if s == "" {
			return nil, nil
		}
		return s, nil
	case "object":
		if s == "" {
			return nil, nil
		}
		return strconv.ParseInt(s, 10, 64)
	case "multiselect", "multiobject", "json":
		if s == "" {
			return nil, nil
		}
		return Decode([]byte(s))
	}
	return s, nil
}

func choiceValue(raw any) (any, error) {
	switch t := raw.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		v, ok := t["value"]
		if !ok {
			return nil, fmt.Errorf("choice object without value: %v", t)
		}
		if v == nil {
			return nil, nil
		}
		return scalarString(v)
	default:
		return scalarString(raw)
	}
}

func refID(raw any) (any, error) {
	switch t := raw.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		id, ok := t["id"]
		if !ok {
			return nil, fmt.Errorf("nested object without id: %v", t)
		}
		return toInt64(id)
	default:
		return toInt64(raw)
	}
}

func list(f *model.Field, raw any) ([]any, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, typeErr(f, "list", raw)
	}
	return items, nil
}

func scalarString(raw any) (string, error) {
	switch t := raw.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	}
	return "", fmt.Errorf("expected a scalar, got %T", raw)
}

func toInt64(raw any) (int64, error) {
	switch t := raw.(type) {
	case json.Number:
		return t.Int64()
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case float64:
		if t != float64(int64(t)) {
			return 0, fmt.Errorf("expected an integer, got %v", t)
		}
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	}
	return 0, fmt.Errorf("expected an integer, got %T", raw)
}

func toFloat64(raw any) (float64, error) {
	switch t := raw.(type) {
	case json.Number:
		return t.Float64()
	case float64:
		return t, nil
	case int64:
		return float64(t), nil
	case string:
		return strconv.ParseFloat(t, 64)
	}
	return 0, fmt.Errorf("expected a number, got %T", raw)
}

// canonicalJSON renders v compactly with sorted object keys, the same form
// Terraform's jsonencode produces.
func canonicalJSON(v any) (string, error) {
	return marshal(v)
}

func marshal(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n")), nil
}

func typeErr(f *model.Field, want string, got any) error {
	return fmt.Errorf("field %q: expected %s, got %T", f.Name, want, got)
}
