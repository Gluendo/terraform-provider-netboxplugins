package detect

import (
	"strings"
	"testing"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
)

func TestCheck(t *testing.T) {
	p := &model.Plugin{
		Package:   "netbox-plugin-example",
		StatusKey: "netbox_example",
		Compatibility: []model.Compatibility{
			{NetBox: ">= 4.6.0, < 4.7.0", Plugin: ">= 1.6.0, < 1.7.0"},
			{NetBox: ">= 4.7.0, < 4.8.0", Plugin: ">= 1.7.0, < 1.8.0"},
		},
	}
	tests := []struct {
		name          string
		info          Info
		installed     bool
		supported     bool
		detailContain string
	}{
		{"missing", Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"other": "1.0"}}, false, false, ""},
		{"supported", Info{NetBoxVersion: "4.7.5", Plugins: map[string]string{"netbox_example": "1.7.3"}}, true, true, ""},
		{"docker suffix", Info{NetBoxVersion: "4.6.10-Docker-5.0.2", Plugins: map[string]string{"netbox_example": "1.6.2"}}, true, true, ""},
		{"plugin too new", Info{NetBoxVersion: "4.6.10", Plugins: map[string]string{"netbox_example": "1.7.3"}}, true, false, "not among the tested"},
		{"netbox too new", Info{NetBoxVersion: "4.8.0", Plugins: map[string]string{"netbox_example": "1.7.3"}}, true, false, "NetBox >= 4.7.0"},
		{"bad version", Info{NetBoxVersion: "main", Plugins: map[string]string{"netbox_example": "1.7.3"}}, true, false, "cannot parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Check(p, tt.info)
			if r.Installed != tt.installed || r.Supported != tt.supported {
				t.Fatalf("got installed=%v supported=%v (%s)", r.Installed, r.Supported, r.Detail)
			}
			if !strings.Contains(r.Detail, tt.detailContain) {
				t.Fatalf("detail %q does not contain %q", r.Detail, tt.detailContain)
			}
		})
	}
}
