data "netboxplugins_dns_view" "default" {
  filters = { name = "_default_" }
}

resource "netboxplugins_dns_nameserver" "ns1" {
  name = "ns1.example.com"
}

resource "netboxplugins_dns_zone" "example" {
  name        = "example.com"
  view        = data.netboxplugins_dns_view.default.id
  nameservers = [netboxplugins_dns_nameserver.ns1.id]
  soa_mname   = netboxplugins_dns_nameserver.ns1.id
  soa_rname   = "hostmaster.example.com"
  status      = "active"
  default_ttl = 3600
}
