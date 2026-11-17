package dns_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/acctest"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins/dns"
)

// config builds a DNS setup touching every model of the plugin. Tags come
// from the core provider (ADR-0004).
func config(n string, o, ttl int, value string) string {
	return acctest.CoreProviderBlock + fmt.Sprintf(`
resource "netbox_tag" "dns" {
  name = "%[1]s-tag"
}

data "netboxplugins_dns_view" "default" {
  filters = { name = "_default_" }
}

resource "netboxplugins_dns_view" "internal" {
  name        = "%[1]s-internal"
  description = "acceptance test"
  tags        = [netbox_tag.dns.name]
}

resource "netboxplugins_dns_nameserver" "ns1" {
  name = "ns1.%[1]s.example.com"
}

resource "netboxplugins_dns_nameserver" "ns2" {
  name = "ns2.%[1]s.example.com"
}

resource "netboxplugins_dns_registrar" "registrar" {
  name         = "%[1]s-registrar"
  iana_id      = %[2]d
  whois_server = "whois.example.com"
}

resource "netboxplugins_dns_registration_contact" "admin" {
  contact_id = "%[1]s-admin"
  name       = "Hostmaster"
  email      = "hostmaster@example.com"
}

resource "netboxplugins_dns_zone" "zone" {
  name        = "%[1]s.example.com"
  view        = netboxplugins_dns_view.internal.id
  nameservers = [netboxplugins_dns_nameserver.ns1.id, netboxplugins_dns_nameserver.ns2.id]
  soa_mname   = netboxplugins_dns_nameserver.ns1.id
  soa_rname   = "hostmaster.%[1]s.example.com"
  status      = "active"
  default_ttl = %[3]d
  registrar   = netboxplugins_dns_registrar.registrar.id
  admin_c     = netboxplugins_dns_registration_contact.admin.id
  tags        = [netbox_tag.dns.name]
}

resource "netboxplugins_dns_record" "www" {
  zone  = netboxplugins_dns_zone.zone.id
  name  = "www"
  type  = "A"
  value = "%[4]s"
  ttl   = 300
}

resource "netboxplugins_dns_record_template" "mx" {
  name        = "%[1]s-mx"
  record_name = "@"
  type        = "MX"
  value       = "10 mail.example.com."
}

resource "netboxplugins_dns_zone_template" "template" {
  name             = "%[1]s-template"
  nameservers      = [netboxplugins_dns_nameserver.ns1.id]
  record_templates = [netboxplugins_dns_record_template.mx.id]
  soa_mname        = netboxplugins_dns_nameserver.ns1.id
  soa_rname        = "hostmaster.example.com"
}

resource "netboxplugins_dns_dnssec_key_template" "ksk" {
  name      = "%[1]s-ksk"
  type      = "KSK"
  algorithm = "ECDSAP256SHA256"
}

resource "netboxplugins_dns_dnssec_policy" "policy" {
  name          = "%[1]s-policy"
  status        = "active"
  key_templates = [netboxplugins_dns_dnssec_key_template.ksk.id]
}

data "netboxplugins_dns_zone" "zone" {
  id = netboxplugins_dns_zone.zone.id
}
`, n, 9000+o, ttl, value)
}

func TestAccDNS(t *testing.T) {
	n := acctest.Name("dns")
	o := acctest.Octet()
	addr := fmt.Sprintf("192.0.2.%d", o)
	cfg := config(n, o, 3600, addr)
	var zoneID, recordID, ns2ID string
	capture := func(dst *string) func(string) error {
		return func(v string) error { *dst = v; return nil }
	}
	path := func(model, id string) string {
		n, _ := strconv.ParseInt(id, 10, 64)
		return dns.Plugin.ObjectPath(dns.Plugin.Model(model), n)
	}

	steps := []resource.TestStep{
		// Create. Adding the record bumps the zone serial in NetBox; the
		// empty plan the framework checks after apply proves that a value
		// the plugin computes produces no diff (ADR-0006).
		{
			Config: cfg,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrWith("netboxplugins_dns_zone.zone", "id", capture(&zoneID)),
				resource.TestCheckResourceAttrWith("netboxplugins_dns_record.www", "id", capture(&recordID)),
				resource.TestCheckResourceAttrWith("netboxplugins_dns_nameserver.ns1", "id", capture(&ns2ID)),
				resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "nameservers.#", "2"),
				resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "status", "active"),
				resource.TestCheckResourceAttrSet("netboxplugins_dns_zone.zone", "soa_serial"),
				resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "tags.#", "1"),
				resource.TestCheckResourceAttrPair("netboxplugins_dns_zone.zone", "registrar", "netboxplugins_dns_registrar.registrar", "id"),
				resource.TestCheckResourceAttr("netboxplugins_dns_record.www", "value", addr),
				resource.TestCheckResourceAttrSet("netboxplugins_dns_record.www", "fqdn"),
				resource.TestCheckResourceAttr("netboxplugins_dns_zone_template.template", "record_templates.#", "1"),
				resource.TestCheckResourceAttr("netboxplugins_dns_dnssec_policy.policy", "key_templates.#", "1"),
				resource.TestCheckResourceAttr("data.netboxplugins_dns_view.default", "name", "_default_"),
				resource.TestCheckResourceAttr("data.netboxplugins_dns_view.default", "default_view", "true"),
				resource.TestCheckResourceAttrPair("data.netboxplugins_dns_zone.zone", "default_ttl", "netboxplugins_dns_zone.zone", "default_ttl"),
			),
		},
	}
	for _, a := range []string{
		"netboxplugins_dns_view.internal", "netboxplugins_dns_nameserver.ns1",
		"netboxplugins_dns_registrar.registrar", "netboxplugins_dns_registration_contact.admin",
		"netboxplugins_dns_zone.zone", "netboxplugins_dns_record.www",
		"netboxplugins_dns_record_template.mx", "netboxplugins_dns_zone_template.template",
		"netboxplugins_dns_dnssec_key_template.ksk", "netboxplugins_dns_dnssec_policy.policy",
	} {
		steps = append(steps, resource.TestStep{
			Config: cfg, ResourceName: a, ImportState: true, ImportStateVerify: true,
			// The zone serial moves when records change; it is not compared.
			ImportStateVerifyIgnore: []string{"soa_serial"},
		})
	}
	// Changes made outside Terraform: reference set, plain choice, integer,
	// tags, and a record value.
	steps = append(steps, acctest.DriftSteps(cfg, func() {
		ns, _ := strconv.ParseInt(ns2ID, 10, 64)
		acctest.Patch(t, path("zone", zoneID), map[string]any{
			"nameservers": []int64{ns},
			"status":      "parked",
			"default_ttl": 60,
			"tags":        []any{},
		})
		acctest.Patch(t, path("record", recordID), map[string]any{"value": "198.51.100.1", "ttl": 900})
	}, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "nameservers.#", "2"),
		resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "status", "active"),
		resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "default_ttl", "3600"),
		resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "tags.#", "1"),
		resource.TestCheckResourceAttr("netboxplugins_dns_record.www", "value", addr),
		resource.TestCheckResourceAttr("netboxplugins_dns_record.www", "ttl", "300"),
	))...)
	// Update in place.
	newAddr := fmt.Sprintf("198.51.100.%d", o)
	steps = append(steps, resource.TestStep{
		Config: config(n, o, 7200, newAddr),
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("netboxplugins_dns_zone.zone", "default_ttl", "7200"),
			resource.TestCheckResourceAttr("netboxplugins_dns_record.www", "value", newAddr),
		),
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.Factories(),
		ExternalProviders:        acctest.CoreProvider(),
		Steps:                    steps,
	})
}
