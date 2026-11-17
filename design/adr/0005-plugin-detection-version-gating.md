# ADR-0005: Plugin detection and version gating

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

A plugin may be missing from a NetBox instance, or installed at a version whose API differs from the one tested. `/api/status/` returns the NetBox version and every installed plugin with its version.

## Options considered

### Option 1: No check

**Description**: Failures surface as 404s or validation errors from NetBox.

| Advantages | Disadvantages                                     |
| ---------- | ------------------------------------------------- |
| + Nothing to maintain | - Errors that do not name the cause    |

### Option 2: Refuse any version outside the tested ranges

**Description**: Error when NetBox or the plugin is outside the declared ranges.

| Advantages      | Disadvantages                                          |
| --------------- | ------------------------------------------------------ |
| + Strict        | - Blocks users on a patch release we have not tested   |

### Option 3: Error when missing, warning when out of range

**Description**: The provider reads `/api/status/` once when configured. A resource whose plugin is not installed fails with an error naming the plugin. A NetBox or plugin version outside the plugin's declared ranges produces a warning.

| Advantages                              | Disadvantages                                |
| --------------------------------------- | -------------------------------------------- |
| + Clear errors                          | - Ranges must be kept current by plugin owners |
| + Untested versions remain usable       |                                              |

## Decision

> **Decision**: Option 3 - Error when missing, warning when out of range

**Justification**: The missing-plugin case is common and confusing without a check; version mismatches are usually harmless and users need a way forward.

Each `plugin.yaml` lists compatibility entries, each a NetBox version range and a plugin version range. `skip_version_check` silences the warnings.

## Consequences

### Positive

- Errors name the missing plugin and the server.

### Negative

- One extra API call when the provider is configured.

### Risks

- Ranges that lag behind releases produce warnings; acceptance tests for a new version update them.

## Actions

- `internal/core/detect` reads `/api/status/` and evaluates ranges.
- Acceptance tests run for every declared range.

## References

- [NetBox status endpoint](https://netboxlabs.com/docs/netbox/integrations/rest-api/)
