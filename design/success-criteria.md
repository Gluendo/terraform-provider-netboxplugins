# Success criteria for v0.1.0

v0.1.0 ships two plugin folders, `dns` (netbox-plugin-dns) and `dataflows` (netbox-data-flows). It is done when every criterion below holds, each with the evidence named.

## Functional

| ID   | Criterion                                                                                                                                                                                  | Evidence                                   |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------ |
| SC1  | Every writable model of both plugins is available as a resource and as a data source, except endpoints that re-expose core NetBox objects (netbox-plugin-dns `/prefixes/`).                                                                                                     | Provider schema lists them                 |
| SC2  | Every resource supports create, read, update, delete and import, and the plan is empty right after apply.                                                                                 | Acceptance tests                           |
| SC3  | Changes made outside Terraform are detected on every field kind: scalar, choice, reference, reference set, list, tags, custom fields. The next apply restores the configuration.        | Acceptance tests with API changes          |
| SC4  | The provider works in the same configuration as `e-breuninger/netbox`, with references by ID and the same environment variables.                                                         | Acceptance tests use both providers        |
| SC5  | A resource whose plugin is missing fails with an error naming the plugin; a version outside the declared ranges produces a warning.                                                     | Unit tests and one acceptance test         |
| SC6  | Only configured attributes are managed ([ADR-0006](adr/0006-configured-attributes-only.md)): server defaults and plugin-computed values never produce a diff.                            | Acceptance tests (DNS zone serial)         |

## Compatibility

| ID   | Criterion                                                                                                    | Evidence                     |
| ---- | ------------------------------------------------------------------------------------------------------------ | ---------------------------- |
| SC7  | Acceptance tests pass on NetBox 4.6 with netbox-plugin-dns 1.6 and netbox-data-flows 1.5.                   | `testenv` profile `netbox-4.6` |
| SC8  | Acceptance tests pass on NetBox 4.7 with netbox-plugin-dns 1.7 and netbox-data-flows 1.6.                   | `testenv` profile `netbox-4.7` |
| SC9  | Acceptance tests pass with OpenTofu and with Terraform.                                                     | Test runs with both binaries |

## Genericity

| ID   | Criterion                                                                                                                                                | Evidence                               |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| SC10 | `internal/core` contains no plugin-specific code.                                                                                                       | No plugin name appears in `internal/core` |
| SC11 | A plugin folder holds `plugin.yaml`, `openapi.json`, generated code and tests only; any hand-written code is an override declared in `plugin.yaml`.      | Folder contents                        |
| SC12 | `go generate ./...` reproduces every generated file with no change.                                                                                     | CI check                               |

## Quality

| ID   | Criterion                                                                          | Evidence             |
| ---- | ---------------------------------------------------------------------------------- | -------------------- |
| SC13 | Every field kind has unit tests for both conversion directions.                    | `go test ./...`      |
| SC14 | `golangci-lint run` reports nothing.                                               | CI                   |
| SC15 | Every resource and data source has generated documentation.                        | `docs/` (tfplugindocs) |

## Real-world use

| ID   | Criterion                                                                                                                                                                                         | Evidence        |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| SC16 | A production-sized flow matrix (26 flows, 17 aliases mixing IP addresses, prefixes and ranges, disabled groups, a select custom field) applies through the provider, and the next plan is empty. | Local test run  |

## Out of scope for v0.1.0

- The generic fallback resource ([ADR-0009](adr/0009-generic-fallback-resource.md)).
- Publication on the Terraform and OpenTofu registries.
- Plugins other than `dns` and `dataflows`.
