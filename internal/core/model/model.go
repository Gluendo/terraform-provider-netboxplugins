// Package model describes NetBox plugin models for the generic engine.
// Plugin folders hold generated values of these types; the engine reads them.
package model

import (
	"fmt"
	"strconv"
)

// Kind is how a field is represented in Terraform and converted to and from
// the NetBox API.
type Kind string

// Field kinds. See ADR-0002.
const (
	KindString       Kind = "string"
	KindInt          Kind = "int"
	KindFloat        Kind = "float"
	KindBool         Kind = "bool"
	KindChoice       Kind = "choice"        // read as {value, label}, written as the value
	KindRef          Kind = "ref"           // read as a nested object, written as its ID
	KindRefSet       Kind = "ref_set"       // read as nested objects, written as IDs
	KindIntList      Kind = "int_list"      // list of integers, same on read and write
	KindStringList   Kind = "string_list"   // list of strings, same on read and write
	KindTags         Kind = "tags"          // read as tag objects, written as names
	KindCustomFields Kind = "custom_fields" // custom field name to value
	KindJSON         Kind = "json"          // any JSON value, as a JSON string
)

// Field is one attribute of a model.
type Field struct {
	Name        string // Terraform attribute name
	API         string // API field name
	Kind        Kind
	Required    bool // required on create
	ReadOnly    bool // set by the server; exported as a computed attribute
	WriteOnly   bool // accepted on write, never returned by the server
	Set         bool // int_list or string_list stored as a set
	Sensitive   bool
	Choices     []string
	Description string
}

// Writable reports whether the field is sent to the API.
func (f *Field) Writable() bool { return !f.ReadOnly }

// Model is one plugin endpoint, exposed as a resource and a data source.
type Model struct {
	Name        string // resource suffix: netboxplugins_<plugin>_<name>
	Endpoint    string // path segment under /api/plugins/<prefix>/
	Description string
	Fields      []Field
}

// HasKind reports whether any field of the model has kind k.
func (m *Model) HasKind(k Kind) bool {
	for i := range m.Fields {
		if m.Fields[i].Kind == k {
			return true
		}
	}
	return false
}

// Compatibility is one supported pair of NetBox and plugin version ranges,
// written as go-version constraints such as ">= 4.7.0, < 4.8.0".
type Compatibility struct {
	NetBox string
	Plugin string
}

// Plugin is one NetBox plugin folder.
type Plugin struct {
	Name          string // folder name and resource prefix
	Title         string
	Package       string // pip package name
	StatusKey     string // key under "plugins" in /api/status/
	APIPrefix     string // /api/plugins/<APIPrefix>/
	Maturity      string // experimental or stable
	Compatibility []Compatibility
	Models        []Model
}

// ListPath is the collection path of a model.
func (p *Plugin) ListPath(m *Model) string {
	return fmt.Sprintf("/api/plugins/%s/%s/", p.APIPrefix, m.Endpoint)
}

// ObjectPath is the path of one object of a model.
func (p *Plugin) ObjectPath(m *Model, id int64) string {
	return p.ListPath(m) + strconv.FormatInt(id, 10) + "/"
}
