// Command nbgen writes a plugin folder's zz_generated.go from its plugin.yaml
// and openapi.json (ADR-0003). Run it from the plugin folder, usually through
// `go generate`.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// pluginFile is the hand-written plugin.yaml.
type pluginFile struct {
	Name          string `yaml:"name"`
	Title         string `yaml:"title"`
	Package       string `yaml:"package"`
	StatusKey     string `yaml:"status_key"`
	APIPrefix     string `yaml:"api_prefix"`
	Maturity      string `yaml:"maturity"`
	Compatibility []struct {
		NetBox string `yaml:"netbox"`
		Plugin string `yaml:"plugin"`
	} `yaml:"compatibility"`
	Models []modelEntry `yaml:"models"`
}

type modelEntry struct {
	Endpoint    string                   `yaml:"endpoint"`
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Overrides   map[string]fieldOverride `yaml:"overrides"`
}

// fieldOverride changes what nbgen derived for one field.
type fieldOverride struct {
	Skip        bool   `yaml:"skip"`
	Name        string `yaml:"name"`
	Kind        string `yaml:"kind"`
	Set         *bool  `yaml:"set"`
	Sensitive   *bool  `yaml:"sensitive"`
	ReadOnly    *bool  `yaml:"read_only"`
	Volatile    *bool  `yaml:"volatile"`
	Description string `yaml:"description"`
}

func main() {
	dir := flag.String("dir", ".", "plugin folder holding plugin.yaml and openapi.json")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "nbgen:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	var pf pluginFile
	if err := readYAML(filepath.Join(dir, "plugin.yaml"), &pf); err != nil {
		return err
	}
	var doc schemaDoc
	raw, err := os.ReadFile(filepath.Join(dir, "openapi.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("openapi.json: %w", err)
	}
	p, err := build(&pf, &doc)
	if err != nil {
		return err
	}
	src, err := render(filepath.Base(absDir(dir)), p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "zz_generated.go"), src, 0o644)
}

func build(pf *pluginFile, doc *schemaDoc) (*model.Plugin, error) {
	p := &model.Plugin{
		Name: pf.Name, Title: pf.Title, Package: pf.Package, StatusKey: pf.StatusKey,
		APIPrefix: pf.APIPrefix, Maturity: pf.Maturity,
	}
	for _, c := range pf.Compatibility {
		p.Compatibility = append(p.Compatibility, model.Compatibility{NetBox: c.NetBox, Plugin: c.Plugin})
	}
	for _, me := range pf.Models {
		m, err := buildModel(pf, doc, me)
		if err != nil {
			return nil, err
		}
		p.Models = append(p.Models, *m)
	}
	return p, nil
}

func buildModel(pf *pluginFile, doc *schemaDoc, me modelEntry) (*model.Model, error) {
	listPath := fmt.Sprintf("/api/plugins/%s/%s/", pf.APIPrefix, me.Endpoint)
	write, read, err := doc.operationSchemas(listPath)
	if err != nil {
		return nil, err
	}
	m := &model.Model{Name: me.Name, Endpoint: me.Endpoint, Description: me.Description}
	if m.Description == "" {
		m.Description = fmt.Sprintf("%s %s.", pf.Title, strings.ReplaceAll(me.Name, "_", " "))
	}
	required := requiredSet(write)
	writable, readOnly := propertyOrder(write, read)
	seen := map[string]bool{}

	add := func(f model.Field) error {
		ov, has := me.Overrides[f.API]
		if has {
			if ov.Skip {
				return nil
			}
			if ov.Name != "" {
				f.Name = ov.Name
			}
			if ov.Kind != "" {
				f.Kind = model.Kind(ov.Kind)
			}
			if ov.Set != nil {
				f.Set = *ov.Set
			}
			if ov.Sensitive != nil {
				f.Sensitive = *ov.Sensitive
			}
			if ov.Volatile != nil {
				f.Volatile = *ov.Volatile
			}
			if ov.ReadOnly != nil {
				f.ReadOnly = *ov.ReadOnly
				if f.ReadOnly {
					f.Required = false
				}
			}
			if ov.Description != "" {
				f.Description = ov.Description
			}
		}
		if reservedNames[f.Name] {
			return fmt.Errorf("%s.%s: %q is reserved by Terraform; set overrides.%s.name", me.Name, f.API, f.Name, f.API)
		}
		if seen[f.Name] {
			return fmt.Errorf("%s: duplicate attribute %q", me.Name, f.Name)
		}
		seen[f.Name] = true
		m.Fields = append(m.Fields, f)
		return nil
	}

	for _, name := range writable {
		wn := doc.resolve(asMap(write.props()[name]))
		rraw, hasRead := read.props()[name]
		rn := doc.resolve(asMap(rraw))
		kind, choices, err := doc.classifyWritable(name, wn, rn, hasRead)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", listPath, err)
		}
		f := model.Field{
			Name: name, API: name, Kind: kind, Required: required[name],
			WriteOnly: !hasRead, Choices: choices,
			Description: description(asMap(write.props()[name]), asMap(rraw)),
		}
		if wb, _ := asMap(write.props()[name])["writeOnly"].(bool); wb {
			f.WriteOnly = true
		}
		if err := add(f); err != nil {
			return nil, err
		}
	}
	for _, name := range readOnly {
		rn := doc.resolve(asMap(read.props()[name]))
		kind, ok := doc.classifyReadOnly(name, rn)
		if !ok {
			continue
		}
		f := model.Field{Name: name, API: name, Kind: kind, ReadOnly: true, Description: description(asMap(read.props()[name]))}
		if err := add(f); err != nil {
			return nil, err
		}
	}
	for name := range me.Overrides {
		if _, ok := write.props()[name]; ok {
			continue
		}
		if _, ok := read.props()[name]; ok {
			continue
		}
		return nil, fmt.Errorf("%s: override for unknown field %q", me.Name, name)
	}
	return m, nil
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

func absDir(dir string) string {
	a, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return a
}

func readYAML(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

var kindConst = map[model.Kind]string{
	model.KindString: "model.KindString", model.KindInt: "model.KindInt", model.KindFloat: "model.KindFloat",
	model.KindBool: "model.KindBool", model.KindChoice: "model.KindChoice", model.KindRef: "model.KindRef",
	model.KindRefSet: "model.KindRefSet", model.KindIntList: "model.KindIntList",
	model.KindStringList: "model.KindStringList", model.KindTags: "model.KindTags",
	model.KindCustomFields: "model.KindCustomFields", model.KindJSON: "model.KindJSON",
}

var tmpl = template.Must(template.New("gen").Funcs(template.FuncMap{
	"q":    func(s string) string { return fmt.Sprintf("%q", s) },
	"kind": func(k model.Kind) string { return kindConst[k] },
	"strs": func(ss []string) string {
		q := make([]string, len(ss))
		for i, s := range ss {
			q[i] = fmt.Sprintf("%q", s)
		}
		return "[]string{" + strings.Join(q, ", ") + "}"
	},
}).Parse(`// Code generated by nbgen from plugin.yaml and openapi.json. DO NOT EDIT.

package {{.Pkg}}

import "github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"

// Plugin describes {{.P.Package}} for the provider.
var Plugin = &model.Plugin{
	Name:      {{q .P.Name}},
	Title:     {{q .P.Title}},
	Package:   {{q .P.Package}},
	StatusKey: {{q .P.StatusKey}},
	APIPrefix: {{q .P.APIPrefix}},
	Maturity:  {{q .P.Maturity}},
	Compatibility: []model.Compatibility{
{{- range .P.Compatibility}}
		{NetBox: {{q .NetBox}}, Plugin: {{q .Plugin}}},
{{- end}}
	},
	Models: []model.Model{
{{- range .P.Models}}
		{
			Name:        {{q .Name}},
			Endpoint:    {{q .Endpoint}},
			Description: {{q .Description}},
			Fields: []model.Field{
{{- range .Fields}}
				{Name: {{q .Name}}, API: {{q .API}}, Kind: {{kind .Kind}}
				{{- if .Required}}, Required: true{{end}}
				{{- if .ReadOnly}}, ReadOnly: true{{end}}
				{{- if .WriteOnly}}, WriteOnly: true{{end}}
				{{- if .Set}}, Set: true{{end}}
				{{- if .Volatile}}, Volatile: true{{end}}
				{{- if .Sensitive}}, Sensitive: true{{end}}
				{{- if .Choices}}, Choices: {{strs .Choices}}{{end}}
				{{- if .Description}}, Description: {{q .Description}}{{end}}},
{{- end}}
			},
		},
{{- end}}
	},
}
`))

func render(pkg string, p *model.Plugin) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		Pkg string
		P   *model.Plugin
	}{pkg, p}); err != nil {
		return nil, err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated code: %w\n%s", err, buf.String())
	}
	return src, nil
}
