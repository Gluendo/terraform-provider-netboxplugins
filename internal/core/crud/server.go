// Package crud is the generic resource and data source engine (ADR-0003).
// Every plugin model is served by the same code, driven by its descriptor.
package crud

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/client"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/detect"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/shape"
)

// ServerData is what the provider hands to every resource and data source.
type ServerData struct {
	Client           *client.Client
	Info             detect.Info
	SkipVersionCheck bool

	warned sync.Map // plugin name -> struct{}

	cfOnce  sync.Once
	cfTypes shape.CustomFieldTypes
	cfErr   error
}

// CheckPlugin returns an error when p is not installed, and a warning, once
// per plugin, when the NetBox and plugin versions are not a tested
// combination (ADR-0005).
func (s *ServerData) CheckPlugin(p *model.Plugin) diag.Diagnostics {
	var diags diag.Diagnostics
	r := detect.Check(p, s.Info)
	if !r.Installed {
		installed := make([]string, 0, len(s.Info.Plugins))
		for k, v := range s.Info.Plugins {
			installed = append(installed, k+" "+v)
		}
		sort.Strings(installed)
		list := strings.Join(installed, ", ")
		if list == "" {
			list = "none"
		}
		diags.AddError(
			fmt.Sprintf("NetBox plugin %s is not installed", p.Package),
			fmt.Sprintf("This configuration uses %s resources, but the NetBox server (version %s) does not report the plugin %q in /api/status/. Installed plugins: %s.",
				p.Title, s.Info.NetBoxVersion, p.StatusKey, list),
		)
		return diags
	}
	if r.Supported || s.SkipVersionCheck {
		return diags
	}
	if _, seen := s.warned.LoadOrStore(p.Name, struct{}{}); !seen {
		diags.AddWarning(
			fmt.Sprintf("Untested %s version", p.Package),
			r.Detail+". Set skip_version_check = true to silence this warning.",
		)
	}
	return diags
}

// CustomFieldTypes returns the type of every custom field defined in NetBox,
// read once per provider run.
func (s *ServerData) CustomFieldTypes(ctx context.Context) (shape.CustomFieldTypes, error) {
	s.cfOnce.Do(func() {
		items, err := s.Client.List(ctx, "/api/extras/custom-fields/", nil, 0)
		if err != nil {
			s.cfErr = fmt.Errorf("read custom field definitions: %w", err)
			return
		}
		s.cfTypes = shape.CustomFieldTypes{}
		for _, it := range items {
			name, _ := it["name"].(string)
			var typ string
			switch t := it["type"].(type) {
			case string:
				typ = t
			case map[string]any:
				typ, _ = t["value"].(string)
			}
			if name != "" {
				s.cfTypes[name] = typ
			}
		}
	})
	return s.cfTypes, s.cfErr
}
