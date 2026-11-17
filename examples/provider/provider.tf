terraform {
  required_providers {
    netbox = {
      source = "e-breuninger/netbox"
    }
    netboxplugins = {
      source = "gluendo/netboxplugins"
    }
  }
}

# Both providers read NETBOX_SERVER_URL and NETBOX_API_TOKEN.
provider "netbox" {}
provider "netboxplugins" {}
