// Package detect reads what a NetBox server runs and checks plugins against
// their declared compatibility (ADR-0005).
package detect

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	version "github.com/hashicorp/go-version"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/client"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

// Info is what /api/status/ reports.
type Info struct {
	NetBoxVersion string
	Plugins       map[string]string // plugin key to version
}

// Fetch reads /api/status/.
func Fetch(ctx context.Context, c *client.Client) (Info, error) {
	v, err := c.Raw(ctx, "GET", "/api/status/")
	if err != nil {
		return Info{}, fmt.Errorf("read NetBox status: %w", err)
	}
	m, _ := v.(map[string]any)
	info := Info{Plugins: map[string]string{}}
	info.NetBoxVersion, _ = m["netbox-version"].(string)
	if plugins, ok := m["plugins"].(map[string]any); ok {
		for k, pv := range plugins {
			if s, ok := pv.(string); ok {
				info.Plugins[k] = s
			}
		}
	}
	if info.NetBoxVersion == "" {
		return Info{}, fmt.Errorf("NetBox status has no netbox-version")
	}
	return info, nil
}

// Result is the outcome of checking one plugin against a server.
type Result struct {
	Installed     bool
	PluginVersion string
	Supported     bool   // a declared compatibility entry matches
	Detail        string // why it is not supported, for the warning
}

// Check evaluates p against info.
func Check(p *model.Plugin, info Info) Result {
	pv, ok := info.Plugins[p.StatusKey]
	if !ok {
		return Result{}
	}
	r := Result{Installed: true, PluginVersion: pv}
	nb, err := parse(info.NetBoxVersion)
	if err != nil {
		r.Detail = fmt.Sprintf("cannot parse NetBox version %q", info.NetBoxVersion)
		return r
	}
	plv, err := parse(pv)
	if err != nil {
		r.Detail = fmt.Sprintf("cannot parse %s version %q", p.Package, pv)
		return r
	}
	var tested []string
	for _, c := range p.Compatibility {
		nbc, err1 := version.NewConstraint(c.NetBox)
		plc, err2 := version.NewConstraint(c.Plugin)
		if err1 != nil || err2 != nil {
			continue
		}
		if nbc.Check(nb) && plc.Check(plv) {
			r.Supported = true
			return r
		}
		tested = append(tested, fmt.Sprintf("NetBox %s with %s %s", c.NetBox, p.Package, c.Plugin))
	}
	sort.Strings(tested)
	r.Detail = fmt.Sprintf("NetBox %s with %s %s is not among the tested combinations: %s",
		info.NetBoxVersion, p.Package, pv, strings.Join(tested, "; "))
	return r
}

var leadingVersion = regexp.MustCompile(`^v?(\d+(\.\d+){0,2})`)

// parse reads the leading x.y.z of a version string; NetBox images may add
// suffixes such as "-Docker-5.1.2".
func parse(s string) (*version.Version, error) {
	m := leadingVersion.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return nil, fmt.Errorf("no version in %q", s)
	}
	return version.NewVersion(m[1])
}
