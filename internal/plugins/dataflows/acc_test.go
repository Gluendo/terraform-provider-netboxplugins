package dataflows_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/acctest"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins/dataflows"
)

// config builds a flow matrix fragment: core objects from the core provider,
// plugin objects from this provider, referenced by ID (ADR-0004).
func config(n string, o int, protocol, ports string) string {
	return acctest.CoreProviderBlock + fmt.Sprintf(`
resource "netbox_tag" "flow" {
  name = "%[1]s-tag"
}

resource "netbox_ip_address" "src" {
  ip_address = "192.0.2.%[5]d/24"
  status     = "active"
  description = "%[1]s"
}

resource "netbox_ip_address" "dst" {
  ip_address = "198.51.100.%[5]d/24"
  status     = "active"
  description = "%[1]s"
}

resource "netbox_prefix" "src" {
  prefix = "203.0.113.%[6]d/26"
  status = "active"
  description = "%[1]s"
}

resource "netbox_ip_range" "dst" {
  start_address = "198.51.100.%[7]d/24"
  end_address   = "198.51.100.%[8]d/24"
  status        = "active"
  description   = "%[1]s"
}

resource "netbox_custom_field_choice_set" "when" {
  name          = "%[1]s-when"
  extra_choices = [{ value = "day1", label = "Day 1" }, { value = "later", label = "Later" }]
}

resource "netbox_custom_field" "when" {
  name          = "%[2]s"
  type          = "select"
  choice_set_id = netbox_custom_field_choice_set.when.id
  object_types  = ["netbox_data_flows.dataflow"]
}

resource "netboxplugins_dataflows_application_role" "infra" {
  name = "%[1]s-infra"
  slug = "%[1]s-infra"
}

resource "netboxplugins_dataflows_application" "app" {
  name        = "%[1]s-app"
  role        = netboxplugins_dataflows_application_role.infra.id
  description = "acceptance test"
}

resource "netboxplugins_dataflows_object_alias" "src" {
  name         = "%[1]s-src"
  ip_addresses = [netbox_ip_address.src.id]
  prefixes     = [netbox_prefix.src.id]
}

resource "netboxplugins_dataflows_object_alias" "dst" {
  name         = "%[1]s-dst"
  ip_addresses = [netbox_ip_address.dst.id]
  ip_ranges    = [netbox_ip_range.dst.id]
}

resource "netboxplugins_dataflows_group" "parent" {
  name        = "%[1]s-parent"
  slug        = "%[1]s-parent"
  application = netboxplugins_dataflows_application.app.id
}

resource "netboxplugins_dataflows_group" "conditional" {
  name   = "%[1]s-conditional"
  slug   = "%[1]s-conditional"
  parent = netboxplugins_dataflows_group.parent.id
  status = "disabled"
}

resource "netboxplugins_dataflows_flow" "web" {
  name              = "%[1]s-web"
  application       = netboxplugins_dataflows_application.app.id
  group             = netboxplugins_dataflows_group.conditional.id
  sources           = [netboxplugins_dataflows_object_alias.src.id]
  destinations      = [netboxplugins_dataflows_object_alias.dst.id]
  protocol          = "%[3]s"
  destination_ports = %[4]s
  status            = "enabled"
  tags              = [netbox_tag.flow.name]
  custom_fields     = { (netbox_custom_field.when.name) = "day1" }
}

data "netboxplugins_dataflows_flow" "web" {
  filters = { name = netboxplugins_dataflows_flow.web.name }
}
`, n, "acc_"+n[len(n)-8:], protocol, ports, o, (o%4)*64, rangeStart(o), rangeStart(o)+5)
}

// rangeStart keeps the IP range clear of the destination address o.
func rangeStart(o int) int {
	if o < 128 {
		return 200
	}
	return 20
}

func TestAccDataFlows(t *testing.T) {
	n := acctest.Name("df")
	o := acctest.Octet()
	cf := "acc_" + n[len(n)-8:]
	cfg := config(n, o, "tcp", "[443, 8443]")
	var flowID, srcID, dstID string
	capture := func(dst *string) func(string) error {
		return func(v string) error { *dst = v; return nil }
	}
	flowPath := func() string {
		id, _ := strconv.ParseInt(flowID, 10, 64)
		return dataflows.Plugin.ObjectPath(dataflows.Plugin.Model("flow"), id)
	}
	aliasPath := func() string {
		id, _ := strconv.ParseInt(srcID, 10, 64)
		return dataflows.Plugin.ObjectPath(dataflows.Plugin.Model("object_alias"), id)
	}

	created := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrWith("netboxplugins_dataflows_flow.web", "id", capture(&flowID)),
		resource.TestCheckResourceAttrWith("netboxplugins_dataflows_object_alias.src", "id", capture(&srcID)),
		resource.TestCheckResourceAttrWith("netboxplugins_dataflows_object_alias.dst", "id", capture(&dstID)),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "protocol", "tcp"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "destination_ports.#", "2"),
		resource.TestCheckTypeSetElemAttr("netboxplugins_dataflows_flow.web", "destination_ports.*", "8443"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "sources.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "status", "enabled"),
		// Read-only choice exported from the API: the group is disabled.
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "inherited_status", "disabled-parent"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "tags.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "custom_fields."+cf, "day1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_object_alias.src", "ip_addresses.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_object_alias.src", "prefixes.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_object_alias.dst", "ip_ranges.#", "1"),
		resource.TestCheckResourceAttrPair("data.netboxplugins_dataflows_flow.web", "id", "netboxplugins_dataflows_flow.web", "id"),
		resource.TestCheckResourceAttr("data.netboxplugins_dataflows_flow.web", "protocol", "tcp"),
		resource.TestCheckResourceAttr("data.netboxplugins_dataflows_flow.web", "custom_fields."+cf, "day1"),
	)

	steps := []resource.TestStep{
		// Create; the framework then checks that the plan is empty.
		{Config: cfg, Check: created},
	}
	for _, addr := range []string{
		"netboxplugins_dataflows_application_role.infra",
		"netboxplugins_dataflows_application.app",
		"netboxplugins_dataflows_object_alias.src",
		"netboxplugins_dataflows_group.conditional",
		"netboxplugins_dataflows_flow.web",
	} {
		steps = append(steps, resource.TestStep{Config: cfg, ResourceName: addr, ImportState: true, ImportStateVerify: true})
	}
	// Changes made outside Terraform on every field kind are detected and reverted.
	steps = append(steps, acctest.DriftSteps(cfg, func() {
		dst, _ := strconv.ParseInt(dstID, 10, 64)
		acctest.Patch(t, flowPath(), map[string]any{
			"sources":           []int64{dst},                // ref set
			"status":            "disabled",                  // choice
			"destination_ports": []int{443},                  // int set
			"tags":              []any{},                     // tags
			"custom_fields":     map[string]any{cf: "later"}, // select custom field
		})
		acctest.Patch(t, aliasPath(), map[string]any{"ip_addresses": []int{}}) // alias membership
	}, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "status", "enabled"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "destination_ports.#", "2"),
		resource.TestCheckResourceAttrPair("netboxplugins_dataflows_flow.web", "sources.0", "netboxplugins_dataflows_object_alias.src", "id"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "tags.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "custom_fields."+cf, "day1"),
		resource.TestCheckResourceAttr("netboxplugins_dataflows_object_alias.src", "ip_addresses.#", "1"),
	))...)
	// Update in place.
	steps = append(steps, resource.TestStep{
		Config: config(n, o, "udp", "[53]"),
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "protocol", "udp"),
			resource.TestCheckResourceAttr("netboxplugins_dataflows_flow.web", "destination_ports.#", "1"),
			resource.TestCheckResourceAttrPair("netboxplugins_dataflows_flow.web", "id", "data.netboxplugins_dataflows_flow.web", "id"),
		),
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.Factories(),
		ExternalProviders:        acctest.CoreProvider(),
		Steps:                    steps,
	})
}
