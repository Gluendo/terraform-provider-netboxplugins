package shape

import (
	"reflect"
	"testing"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

func decode(t *testing.T, s string) any {
	t.Helper()
	v, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("decode %s: %v", s, err)
	}
	return v
}

func TestFromAPI(t *testing.T) {
	cft := CustomFieldTypes{
		"when": "select", "count": "integer", "ratio": "decimal", "on": "boolean",
		"colors": "multiselect", "owner": "object", "owners": "multiobject", "meta": "json", "note": "text",
	}
	tests := []struct {
		name string
		f    model.Field
		raw  string
		want any
	}{
		{"string", model.Field{Kind: model.KindString}, `"web"`, "web"},
		{"string null", model.Field{Kind: model.KindString}, `null`, nil},
		{"int", model.Field{Kind: model.KindInt}, `3600`, int64(3600)},
		{"float", model.Field{Kind: model.KindFloat}, `1.5`, 1.5},
		{"bool", model.Field{Kind: model.KindBool}, `true`, true},
		{"choice object", model.Field{Kind: model.KindChoice}, `{"value":"tcp","label":"TCP"}`, "tcp"},
		{"choice plain", model.Field{Kind: model.KindChoice}, `"tcp"`, "tcp"},
		{"choice null", model.Field{Kind: model.KindChoice}, `null`, nil},
		{"ref object", model.Field{Kind: model.KindRef}, `{"id":7,"url":"x","display":"g"}`, int64(7)},
		{"ref null", model.Field{Kind: model.KindRef}, `null`, nil},
		{"ref set", model.Field{Kind: model.KindRefSet}, `[{"id":9},{"id":2}]`, []int64{2, 9}},
		{"ref set empty", model.Field{Kind: model.KindRefSet}, `[]`, []int64{}},
		{"int list keeps order", model.Field{Kind: model.KindIntList}, `[8443,443]`, []int64{8443, 443}},
		{"int set sorted", model.Field{Kind: model.KindIntList, Set: true}, `[8443,443]`, []int64{443, 8443}},
		{"int list null", model.Field{Kind: model.KindIntList}, `null`, nil},
		{"string list", model.Field{Kind: model.KindStringList}, `["b","a"]`, []string{"b", "a"}},
		{"tags", model.Field{Kind: model.KindTags}, `[{"id":1,"name":"prod","slug":"prod"},{"id":2,"name":"edge","slug":"edge"}]`, []string{"edge", "prod"}},
		{"json", model.Field{Kind: model.KindJSON}, `{"b":1,"a":[true]}`, `{"a":[true],"b":1}`},
		{"custom fields", model.Field{Kind: model.KindCustomFields},
			`{"when":{"value":"day1","label":"Day 1"},"count":5,"ratio":"1.50","on":false,"colors":[{"value":"red","label":"Red"}],` +
				`"owner":{"id":4,"display":"x"},"owners":[{"id":6},{"id":5}],"meta":{"k":"v"},"note":"hello","unset":null}`,
			map[string]string{"when": "day1", "count": "5", "ratio": "1.50", "on": "false", "colors": `["red"]`,
				"owner": "4", "owners": "[5,6]", "meta": `{"k":"v"}`, "note": "hello"}},
		{"custom fields select before 4.7", model.Field{Kind: model.KindCustomFields}, `{"when":"day1"}`, map[string]string{"when": "day1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromAPI(&tt.f, decode(t, tt.raw), cft)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestToAPI(t *testing.T) {
	cft := CustomFieldTypes{"when": "select", "count": "integer", "on": "boolean", "colors": "multiselect", "owner": "object", "note": "text"}
	tests := []struct {
		name string
		f    model.Field
		in   any
		want any
	}{
		{"string", model.Field{Kind: model.KindString}, "web", "web"},
		{"choice", model.Field{Kind: model.KindChoice}, "tcp", "tcp"},
		{"ref", model.Field{Kind: model.KindRef}, int64(7), int64(7)},
		{"ref set", model.Field{Kind: model.KindRefSet}, []int64{2, 9}, []int64{2, 9}},
		{"int list", model.Field{Kind: model.KindIntList}, []int64{443}, []int64{443}},
		{"tags", model.Field{Kind: model.KindTags}, []string{"prod"}, []map[string]string{{"name": "prod"}}},
		{"json", model.Field{Kind: model.KindJSON}, `{"a":1}`, map[string]any{"a": decode(t, "1")}},
		{"custom fields", model.Field{Kind: model.KindCustomFields},
			map[string]string{"when": "day1", "count": "5", "on": "true", "colors": `["red"]`, "owner": "4", "note": "hi", "undefined": "x"},
			map[string]any{"when": "day1", "count": int64(5), "on": true, "colors": []any{"red"}, "owner": int64(4), "note": "hi", "undefined": "x"}},
		{"custom field cleared", model.Field{Kind: model.KindCustomFields}, map[string]string{"when": ""}, map[string]any{"when": nil}},
		{"nil", model.Field{Kind: model.KindString}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToAPI(&tt.f, tt.in, cft)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// A value written with ToAPI and read back with FromAPI must be unchanged,
// or every plan would show a diff.
func TestRoundTrip(t *testing.T) {
	cft := CustomFieldTypes{"count": "integer", "colors": "multiselect"}
	fields := []struct {
		f model.Field
		v any
	}{
		{model.Field{Kind: model.KindTags}, []string{"a", "b"}},
		{model.Field{Kind: model.KindJSON}, `{"a":[1,2]}`},
		{model.Field{Kind: model.KindCustomFields}, map[string]string{"count": "5", "colors": `["red","blue"]`}},
	}
	for _, tc := range fields {
		api, err := ToAPI(&tc.f, tc.v, cft)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := marshal(api)
		if err != nil {
			t.Fatal(err)
		}
		back, err := FromAPI(&tc.f, decode(t, raw), cft)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, tc.v) {
			t.Errorf("%s: wrote %#v, read %#v", tc.f.Kind, tc.v, back)
		}
	}
}

func TestFromAPIErrors(t *testing.T) {
	bad := []struct {
		f   model.Field
		raw string
	}{
		{model.Field{Kind: model.KindInt}, `"x"`},
		{model.Field{Kind: model.KindRef}, `{"name":"no id"}`},
		{model.Field{Kind: model.KindBool}, `"yes"`},
		{model.Field{Kind: model.KindRefSet}, `{"id":1}`},
	}
	for _, b := range bad {
		if _, err := FromAPI(&b.f, decode(t, b.raw), nil); err == nil {
			t.Errorf("%s from %s: expected an error", b.f.Kind, b.raw)
		}
	}
}
