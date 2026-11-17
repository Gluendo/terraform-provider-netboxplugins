# ADR-0003: Generated model descriptors and one generic engine

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

Each plugin model needs a Terraform schema and create, read, update, delete and import logic. The provider uses the Terraform Plugin Framework (protocol 6).

## Options considered

### Option 1: Hand-written resources

**Description**: One Go file per model.

| Advantages     | Disadvantages                                       |
| -------------- | --------------------------------------------------- |
| + Simple start | - Code grows with every model of every plugin       |
|                | - Fixes repeated in every file                      |

### Option 2: Full resource code from HashiCorp's code generators

**Description**: `tfplugingen-openapi` and `tfplugingen-framework` generate schema and model code from OpenAPI; NetBox-specific behaviour is added with overlays, JSON patches and hand-written CRUD, as `ffddorf/netbox-bgp` does.

| Advantages                     | Disadvantages                                                      |
| ------------------------------ | ------------------------------------------------------------------ |
| + Proven on one NetBox plugin  | - The generators are labelled experimental                         |
|                                | - They know nothing of NetBox's read/write formats                 |
|                                | - CRUD still written per resource                                  |

### Option 3: Own generator producing descriptors for a generic engine

**Description**: `nbgen` reads a plugin's `plugin.yaml` and `openapi.json` and writes a data descriptor per model: endpoint, fields, kinds, required flags, choices, descriptions. `internal/core/crud` implements one resource and one data source that work from any descriptor.

| Advantages                                         | Disadvantages                                                  |
| -------------------------------------------------- | -------------------------------------------------------------- |
| + Engine fixes apply to every plugin               | - Behaviour specific to one plugin needs an extension point    |
| + Generated code is small data, easy to review     | - The generator is ours to maintain                            |

## Decision

> **Decision**: Option 3 - Own generator producing descriptors for a generic engine

**Justification**: NetBox plugin models follow one REST convention, so one engine covers them; the generator only has to describe each model.

## Consequences

### Positive

- A plugin folder is `plugin.yaml`, `openapi.json`, one generated file and its tests.

### Negative

- The engine must handle every convention every supported plugin uses.

### Risks

- A plugin endpoint that breaks NetBox conventions cannot be described; such a model stays out until the engine gains an extension point.

## Actions

- `tools/extract` takes a plugin's part of `/api/schema/` from a running NetBox into `openapi.json`.
- `tools/nbgen` writes `zz_generated.go`; CI fails when `go generate ./...` changes any file.

## References

- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [HashiCorp provider code generation](https://developer.hashicorp.com/terraform/plugin/code-generation)
