package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/acctest"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/provider"
)

// A plugin folder whose plugin the server does not run fails with an error
// naming it (ADR-0005, success criterion SC5).
func TestAccMissingPlugin(t *testing.T) {
	absent := &model.Plugin{
		Name: "absent", Title: "Absent plugin", Package: "netbox-plugin-absent", StatusKey: "netbox_absent",
		APIPrefix: "absent", Maturity: "experimental",
		Models: []model.Model{{Name: "thing", Endpoint: "things", Description: "Thing.",
			Fields: []model.Field{{Name: "name", API: "name", Kind: model.KindString, Required: true}}}},
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			provider.TypeName: providerserver.NewProtocol6WithError(provider.New("acctest", []*model.Plugin{absent})()),
		},
		Steps: []resource.TestStep{{
			Config:      `resource "netboxplugins_absent_thing" "x" { name = "x" }`,
			ExpectError: regexp.MustCompile(`NetBox plugin netbox-plugin-absent is not installed`),
		}},
	})
}
