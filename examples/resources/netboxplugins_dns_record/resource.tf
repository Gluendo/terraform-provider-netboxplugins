resource "netboxplugins_dns_record" "www" {
  zone  = netboxplugins_dns_zone.example.id
  name  = "www"
  type  = "A"
  value = "192.0.2.10"
  ttl   = 300
}
