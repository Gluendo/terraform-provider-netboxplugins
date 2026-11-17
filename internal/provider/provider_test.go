package provider

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins"
)

// The framework validates every resource and data source schema when the
// schema is requested; a descriptor that produces an invalid schema fails here.
func TestSchemas(t *testing.T) {
	srv, err := providerserver.NewProtocol6WithError(New("test", plugins.All)())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		t.Errorf("%s: %s: %s", d.Severity, d.Summary, d.Detail)
	}

	models := 0
	for _, p := range plugins.All {
		models += len(p.Models)
		for _, m := range p.Models {
			name := TypeName + "_" + p.Name + "_" + m.Name
			if _, ok := resp.ResourceSchemas[name]; !ok {
				t.Errorf("resource %s missing", name)
			}
			if _, ok := resp.DataSourceSchemas[name]; !ok {
				t.Errorf("data source %s missing", name)
			}
		}
	}
	if len(resp.ResourceSchemas) != models || len(resp.DataSourceSchemas) != models {
		t.Errorf("got %d resources and %d data sources, want %d each", len(resp.ResourceSchemas), len(resp.DataSourceSchemas), models)
	}
}

// internal/core must stay plugin-agnostic (success criterion SC10): no plugin
// identifier and no import of a plugin folder.
func TestCoreIsPluginAgnostic(t *testing.T) {
	var needles []string
	for _, p := range plugins.All {
		needles = append(needles, p.StatusKey, p.APIPrefix, p.Package, "/internal/plugins/"+p.Name)
	}
	err := filepath.WalkDir("../core", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, n := range needles {
			if strings.Contains(string(src), n) {
				t.Errorf("%s mentions %q", path, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
