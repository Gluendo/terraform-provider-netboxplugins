resource "netbox_ip_address" "web" {
  ip_address = "192.0.2.10/24"
  status     = "active"
}

resource "netbox_prefix" "clients" {
  prefix = "198.51.100.0/24"
  status = "active"
}

resource "netboxplugins_dataflows_object_alias" "web" {
  name         = "web-servers"
  ip_addresses = [netbox_ip_address.web.id]
}

resource "netboxplugins_dataflows_object_alias" "clients" {
  name     = "clients"
  prefixes = [netbox_prefix.clients.id]
}
