package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// Every committed zz_generated.go must equal what nbgen produces from the
// committed plugin.yaml and openapi.json (success criterion SC12).
func TestGeneratedFilesUpToDate(t *testing.T) {
	dirs, err := filepath.Glob("../../internal/plugins/*/plugin.yaml")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no plugin folders found: %v", err)
	}
	for _, py := range dirs {
		dir := filepath.Dir(py)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			var pf pluginFile
			if err := readYAML(py, &pf); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "openapi.json"))
			if err != nil {
				t.Fatal(err)
			}
			var doc schemaDoc
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			p, err := build(&pf, &doc)
			if err != nil {
				t.Fatal(err)
			}
			want, err := render(filepath.Base(dir), p)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "zz_generated.go"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s/zz_generated.go is stale; run go generate ./...", dir)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	doc := schemaDoc{}
	doc.Components.Schemas = map[string]map[string]any{
		"BriefThing":        {"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}, "name": map[string]any{"type": "string"}}},
		"BriefThingRequest": {"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}},
		"NestedTag":         {"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}, "name": map[string]any{"type": "string"}}},
	}
	ref := func(n string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + n} }
	choiceRead := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}}}

	tests := []struct {
		name    string
		w, r    map[string]any
		hasRead bool
		want    model.Kind
	}{
		{"ref via oneOf", map[string]any{"oneOf": []any{map[string]any{"type": "integer"}, map[string]any{"allOf": []any{ref("BriefThingRequest")}}}}, map[string]any{"allOf": []any{ref("BriefThing")}}, true, model.KindRef},
		{"ref via integer", map[string]any{"type": "integer"}, ref("BriefThing"), true, model.KindRef},
		{"ref set via ids", map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, map[string]any{"type": "array", "items": ref("BriefThing")}, true, model.KindRefSet},
		{"ref set via request objects", map[string]any{"type": "array", "items": ref("BriefThingRequest")}, map[string]any{"type": "array", "items": ref("BriefThing")}, true, model.KindRefSet},
		{"choice", map[string]any{"type": "string", "enum": []any{"a", "b"}}, choiceRead, true, model.KindChoice},
		{"plain enum", map[string]any{"type": "string", "enum": []any{"a", "b"}}, map[string]any{"type": "string", "enum": []any{"a", "b"}}, true, model.KindString},
		{"int list", map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}, true, model.KindIntList},
		{"string oneOf", map[string]any{"oneOf": []any{map[string]any{"type": "string", "format": "email"}, map[string]any{"type": "string"}}}, nil, true, model.KindString},
		{"json any", map[string]any{"nullable": true}, map[string]any{"nullable": true}, true, model.KindJSON},
		{"write-only ref", map[string]any{"oneOf": []any{map[string]any{"type": "integer"}, ref("BriefThingRequest")}}, nil, false, model.KindRef},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := doc.classifyWritable("field", doc.resolve(tt.w), doc.resolve(tt.r), tt.hasRead)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}
