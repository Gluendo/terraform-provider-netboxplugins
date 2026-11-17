# ADR-0001: One provider, one folder per NetBox plugin

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

NetBox plugins add models under `/api/plugins/<prefix>/`. The core NetBox providers, `e-breuninger/netbox` first among them, manage core models only. Plugin objects are managed today with a provider written for one plugin (`ffddorf/netbox-bgp`, `devon-mar/nblists`) or with a generic REST provider. Each single-plugin provider repeats the same parts: authentication, HTTP handling, conversion between NetBox's read and write formats, test environment, release pipeline.

## Options considered

### Option 1: One provider per plugin

**Description**: Each plugin gets its own repository, binary and registry entry.

| Advantages                          | Disadvantages                                                  |
| ----------------------------------- | -------------------------------------------------------------- |
| + Fully independent lifecycle       | - Shared parts written again in every provider                 |
| + Ownership is obvious              | - Users configure one provider block per plugin                |
|                                     | - A contributor must bootstrap a whole provider for one plugin |

### Option 2: One provider, one folder per plugin

**Description**: One binary. A shared core holds the client, version detection, read/write conversion, generic resource engine and test harness. Each plugin is a folder under `internal/plugins/<name>`, owned by its contributors.

| Advantages                                          | Disadvantages                                         |
| --------------------------------------------------- | ----------------------------------------------------- |
| + Shared parts written and fixed once               | - One release carries every plugin                    |
| + Adding a plugin is adding a folder                | - Core changes must keep every folder working         |
| + One provider block for every plugin               |                                                       |

### Option 3: One generic, untyped resource

**Description**: A single resource takes an API path and a free-form attribute map.

| Advantages                     | Disadvantages                                         |
| ------------------------------ | ----------------------------------------------------- |
| + No per-plugin code           | - No typed attributes, validation or per-model docs   |
| + Works for any plugin at once | - Weak plan readability                               |

### Option 4: Add plugin resources to the core provider

**Description**: Contribute plugin resources upstream to `e-breuninger/netbox`.

| Advantages                    | Disadvantages                                                    |
| ----------------------------- | ---------------------------------------------------------------- |
| + No new provider for users   | - The core provider's scope is core NetBox models                |
|                               | - Plugin releases do not follow NetBox releases                  |
|                               | - Plugin owners do not control their code                        |

## Decision

> **Decision**: Option 2 - One provider, one folder per plugin

**Justification**: The shared parts are most of the work and the source of most bugs; writing them once lets a plugin folder stay small. Option 3 is kept as a fallback resource for plugins without a folder ([ADR-0009](0009-generic-fallback-resource.md)).

Naming: the provider type is `netboxplugins`; resources and data sources are `netboxplugins_<plugin>_<model>`, for example `netboxplugins_dns_zone`.

## Consequences

### Positive

- A plugin folder contains its metadata, its schema, generated code and tests, and nothing else.
- Users configure one provider block for every plugin.

### Negative

- A release ships every plugin, including ones a user does not run.

### Risks

- A core change can break a plugin its author does not test; CI runs every folder when core changes ([ADR-0007](0007-acceptance-tests-real-netbox.md)).

## Actions

- Layout: `internal/core` for shared code, `internal/plugins/<name>` per plugin.
- First two folders: `dns` (netbox-plugin-dns) and `dataflows` (netbox-data-flows).

## References

- [ffddorf/terraform-provider-netbox-bgp](https://github.com/ffddorf/terraform-provider-netbox-bgp)
- [devon-mar/terraform-provider-nblists](https://github.com/devon-mar/terraform-provider-nblists)
- [e-breuninger/terraform-provider-netbox](https://github.com/e-breuninger/terraform-provider-netbox)
