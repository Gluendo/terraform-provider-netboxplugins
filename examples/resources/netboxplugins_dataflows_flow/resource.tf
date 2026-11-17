resource "netboxplugins_dataflows_flow" "https" {
  name              = "clients-to-web"
  sources           = [netboxplugins_dataflows_object_alias.clients.id]
  destinations      = [netboxplugins_dataflows_object_alias.web.id]
  protocol          = "tcp"
  destination_ports = [443]
  status            = "enabled"
}
