package crud

import (
	"strings"
	"testing"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/detect"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

var examplePlugin = &model.Plugin{
	Name: "example", Title: "NetBox Example", Package: "netbox-plugin-example", StatusKey: "netbox_example",
	Compatibility: []model.Compatibility{{NetBox: ">= 4.7.0, < 4.8.0", Plugin: ">= 2.0.0, < 3.0.0"}},
}

func TestCheckPluginMissing(t *testing.T) {
	srv := &ServerData{Info: detect.Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"netbox_other": "1.0"}}}
	diags := srv.CheckPlugin(examplePlugin)
	if !diags.HasError() {
		t.Fatal("expected an error for a missing plugin")
	}
	got := diags.Errors()[0]
	if !strings.Contains(got.Summary(), "netbox-plugin-example") || !strings.Contains(got.Detail(), "netbox_other 1.0") {
		t.Fatalf("error does not name the plugin and the installed ones: %s / %s", got.Summary(), got.Detail())
	}
}

func TestCheckPluginUntestedWarnsOnce(t *testing.T) {
	srv := &ServerData{Info: detect.Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"netbox_example": "3.1.0"}}}
	first := srv.CheckPlugin(examplePlugin)
	if first.HasError() || first.WarningsCount() != 1 {
		t.Fatalf("first check: want one warning, got %v", first)
	}
	if again := srv.CheckPlugin(examplePlugin); again.WarningsCount() != 0 {
		t.Fatalf("second check: want no repeated warning, got %v", again)
	}
}

func TestCheckPluginSkipVersionCheck(t *testing.T) {
	srv := &ServerData{SkipVersionCheck: true, Info: detect.Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"netbox_example": "3.1.0"}}}
	if d := srv.CheckPlugin(examplePlugin); len(d) != 0 {
		t.Fatalf("want no diagnostics, got %v", d)
	}
}

func TestCheckPluginSupported(t *testing.T) {
	srv := &ServerData{Info: detect.Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"netbox_example": "2.4.1"}}}
	if d := srv.CheckPlugin(examplePlugin); len(d) != 0 {
		t.Fatalf("want no diagnostics, got %v", d)
	}
}
