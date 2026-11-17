# ADR-0008: Contribution, ownership and releases

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

Plugins change on their own schedules. Contributors know their plugin, not every plugin.

## Options considered

### Option 1: Central maintainers own every folder

**Description**: Core maintainers review and maintain all plugins.

| Advantages            | Disadvantages                                          |
| --------------------- | ------------------------------------------------------ |
| + One review standard | - Maintainers become the bottleneck for every plugin   |

### Option 2: Each folder owned by its contributors

**Description**: `CODEOWNERS` assigns each `internal/plugins/<name>` folder to its owners; core maintainers own `internal/core` and `tools`.

| Advantages                                  | Disadvantages                                   |
| ------------------------------------------- | ----------------------------------------------- |
| + Plugin experts review their plugin        | - Quality can vary between folders              |
| + Unmaintained folders are visible          |                                                 |

## Decision

> **Decision**: Option 2 - Each folder owned by its contributors

**Justification**: Plugin knowledge sits with plugin users; the core and its tests keep a common quality bar.

- `plugin.yaml` declares a maturity: `experimental` or `stable`.
- A folder without an owner for two consecutive minor releases is marked deprecated, and removed in the next major release.
- One binary, semantic versioning, one changelog section per plugin.
- Licence: MPL-2.0, the licence of the HashiCorp provider ecosystem.

## Consequences

### Positive

- Adding a plugin needs no core maintainer beyond the review of the folder.

### Negative

- Users see maturity per resource, not per release.

### Risks

- Few owners at the start; the first folders are owned by the project maintainers.

## Actions

- `CODEOWNERS` with one line per plugin folder.
- `CONTRIBUTING.md` with the steps to add a plugin.

## References

- [GitHub CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)
- [Mozilla Public License 2.0](https://www.mozilla.org/en-US/MPL/2.0/)
