# Terraform provider for NetBox plugins

Manages objects of [NetBox](https://netboxlabs.com/oss/netbox/) plugins with Terraform or OpenTofu, next to the core [`e-breuninger/netbox`](https://github.com/e-breuninger/terraform-provider-netbox) provider.

One provider covers many plugins. Each plugin is a folder under [`internal/plugins/`](internal/plugins/), generated from the OpenAPI schema NetBox publishes, and owned by the people who use that plugin.

| Plugin                                                                   | Resources                   | Tested with                                   |
| ------------------------------------------------------------------------ | --------------------------- | --------------------------------------------- |
| [netbox-plugin-dns](https://github.com/peteeckel/netbox-plugin-dns)      | `netboxplugins_dns_*`       | NetBox 4.6 with 1.6.2, NetBox 4.7 with 1.7.3   |
| [netbox-data-flows](https://github.com/Alef-Burzmali/netbox-data-flows)  | `netboxplugins_dataflows_*` | NetBox 4.6 with 1.5.4, NetBox 4.7 with 1.6.0   |

Published on the [OpenTofu Registry](https://search.opentofu.org/provider/gluendo/netboxplugins) and the [Terraform Registry](https://registry.terraform.io/providers/Gluendo/netboxplugins/latest), with the documentation of every resource and data source.

Status: experimental. Releases are signed with the key in [`release-signing-key.asc`](release-signing-key.asc) (fingerprint `52579951E963FE16C1FFECF79884577BBC23CBE4`).

## Usage

```hcl
terraform {
  required_providers {
    netbox        = { source = "e-breuninger/netbox" }
    netboxplugins = { source = "gluendo/netboxplugins" }
  }
}

# Both providers read NETBOX_SERVER_URL and NETBOX_API_TOKEN.

resource "netbox_ip_address" "web" {
  ip_address = "192.0.2.10/24"
  status     = "active"
}

resource "netboxplugins_dataflows_object_alias" "web" {
  name         = "web-servers"
  ip_addresses = [netbox_ip_address.web.id]
}

resource "netboxplugins_dns_record" "www" {
  zone  = netboxplugins_dns_zone.example.id
  name  = "www"
  type  = "A"
  value = "192.0.2.10"
}
```

Behaviour shared by every resource:

- Core NetBox objects are referenced by ID; tags by name.
- Only attributes set in the configuration are managed. To clear a value, set it explicitly (`""`, `[]`).
- Changes made in NetBox outside Terraform show in the next plan, on every attribute kind: references, choices, lists, tags, custom fields.
- A resource whose plugin is not installed fails with an error naming the plugin; untested versions work with a warning.
- Import takes the NetBox ID: `terraform import netboxplugins_dns_zone.example 42`.

Resource documentation: [`docs/`](docs/).

## How it works

```text
plugin.yaml + openapi.json ──nbgen──▶ zz_generated.go (model descriptors)
                                            │
internal/core: client · detect · shape · tfschema · crud  ◀── one engine for every model
```

`nbgen` compares the read and write schema of each field to decide how NetBox serialises it, so the engine can compare what it wrote with what it reads back. The design and its trade-offs are in [`design/adr/`](design/adr/); the release goals in [`design/success-criteria.md`](design/success-criteria.md).

## Development

```sh
make test                         # unit tests
make lint                         # golangci-lint
make generate                     # descriptors and docs
make testenv-up PROFILE=netbox-4.7
make testacc PROFILE=netbox-4.7   # acceptance tests with OpenTofu
make testacc-terraform PROFILE=netbox-4.7
make testenv-down PROFILE=netbox-4.7
```

The test environment needs Podman or Docker. Adding a plugin is described in [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

[MPL-2.0](LICENSE).
