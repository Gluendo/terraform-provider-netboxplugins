# ADR-0002: Read/write conversion derived from NetBox's OpenAPI schema

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

NetBox returns an object in a different format from the one it accepts:

| Field type          | Read                                   | Write                    |
| ------------------- | -------------------------------------- | ------------------------ |
| Related object      | Nested object (`{id, url, display...}`) | Integer ID               |
| List of related     | List of nested objects                 | List of integer IDs      |
| Choice              | `{value, label}`                       | Plain value              |
| Tags                | List of tag objects                    | List of `{name}` or IDs  |
| Select custom field | `{value, label}` (NetBox 4.7 and later) | Plain value              |
| Server-set fields   | `id`, `url`, `display`, `created`...   | Not accepted             |

A provider that compares the read format with what it wrote reports a change on every plan. A generic REST provider tested against a plugin did exactly that; with its ignore options configured, plans were clean but changes to references, choices and list membership made outside Terraform went undetected.

NetBox publishes its OpenAPI schema at `/api/schema/`. It includes every installed plugin, and each model has a read schema (for example `DataFlow`) and a write schema (`WritableDataFlowRequest`).

## Options considered

### Option 1: Conversion written by hand for each resource

**Description**: Each resource implements its own mapping.

| Advantages            | Disadvantages                                         |
| --------------------- | ----------------------------------------------------- |
| + Full control        | - The same mapping written for every model            |
|                       | - Mistakes differ from one resource to the next       |

### Option 2: Generic REST resource with ignore lists

**Description**: Store the raw response and ignore fields whose format differs.

| Advantages       | Disadvantages                                          |
| ---------------- | ------------------------------------------------------ |
| + No code        | - Drift on ignored fields is invisible                 |
|                  | - Every server-added field must be listed              |

### Option 3: Conversion derived from the read and write schemas

**Description**: For each field, the generator compares its read and write schemas and assigns a kind. The core converts every kind the same way for every plugin.

| Advantages                                           | Disadvantages                                                   |
| ---------------------------------------------------- | --------------------------------------------------------------- |
| + Correct drift detection on every writable field    | - Fields the server rewrites (for example lower-casing) need an override |
| + A new plugin needs no conversion code              |                                                                 |

## Decision

> **Decision**: Option 3 - Conversion derived from the read and write schemas

**Justification**: The format difference follows NetBox conventions that the schema states explicitly, so it can be handled once.

Kinds: `string`, `int`, `float`, `bool`, `choice`, `ref`, `ref_set`, `int_list`, `string_list`, `tags`, `custom_fields`, `json`. Server-set scalar, choice and reference fields are exported as computed attributes and never compared. Server-set counters (`*_count`) and reverse-relation lists are not exported: they change whenever other objects change.

## Consequences

### Positive

- Changes made outside Terraform are detected on references, choices, lists, tags and custom fields.

### Negative

- A plugin that serialises a field outside NetBox conventions needs an override in its `plugin.yaml`.

### Risks

- A NetBox release that changes a convention (as 4.7 did for select custom fields) needs a core change; acceptance tests on each supported NetBox version catch it.

## Actions

- `internal/core/shape`: conversion per kind, with unit tests for each kind.
- `tools/nbgen`: assigns kinds from read/write schema pairs.

## References

- [NetBox REST API](https://netboxlabs.com/docs/netbox/integrations/rest-api/)
- [NetBox 4.7 release notes](https://netboxlabs.com/docs/netbox/release-notes/version-4.7) (select custom field format)
