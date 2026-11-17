# ADR-0009: Generic fallback resource for plugins without a folder

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

A plugin is only covered once someone writes its folder. Until then, users fall back to generic REST providers, which cannot compare NetBox's read and write formats ([ADR-0002](0002-schema-derived-read-write-conversion.md)).

## Options considered

### Option 1: No fallback

**Description**: Users without a folder use a generic REST provider.

| Advantages            | Disadvantages                                        |
| --------------------- | ---------------------------------------------------- |
| + Nothing to maintain | - Drift on references and choices goes undetected    |

### Option 2: `netboxplugins_object`

**Description**: A resource taking `path` and `attributes` (JSON). The provider reads the live `/api/schema/` when configured and applies the same conversion as typed resources.

| Advantages                                    | Disadvantages                                  |
| --------------------------------------------- | ---------------------------------------------- |
| + Any plugin usable before its folder exists  | - Untyped attributes, no per-model docs        |
| + Same drift detection as typed resources     | - Schema download when the provider starts     |

## Decision

> **Decision**: Option 2 - `netboxplugins_object`

**Justification**: It reuses the conversion core and gives correct drift detection to every plugin from day one.

## Consequences

### Positive

- Users can start before a folder exists, then move to typed resources.

### Negative

- Two ways to manage the same model once a folder exists.

### Risks

- Users stay on the untyped resource; the documentation recommends typed resources where they exist.

## Actions

- Implement after the first typed plugins meet the success criteria.

## References

- [ADR-0002](0002-schema-derived-read-write-conversion.md)
