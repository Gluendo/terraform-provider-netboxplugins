# ADR-0007: Acceptance tests against real NetBox and plugin versions

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

Correctness depends on how each NetBox and plugin version serialises objects. Recorded or mocked responses would not show the format differences of a new version.

## Options considered

### Option 1: Unit tests with recorded responses

**Description**: Replay stored HTTP responses.

| Advantages | Disadvantages                                          |
| ---------- | ------------------------------------------------------ |
| + Fast     | - Recordings go stale silently with each new version   |

### Option 2: Acceptance tests against real NetBox containers

**Description**: Each declared compatibility range has a test environment: the NetBox image with the plugin packages installed. Tests use `terraform-plugin-testing`. Conversion and generator logic keep unit tests.

| Advantages                                  | Disadvantages                              |
| ------------------------------------------- | ------------------------------------------ |
| + Catches format changes of real versions   | - Slower CI, images built per NetBox version |

## Decision

> **Decision**: Option 2 - Acceptance tests against real NetBox containers

**Justification**: The format differences this provider exists to handle only show up against real servers.

Every resource runs the standard sequence: create, empty plan, import, change attributes through the API, plan shows the change, apply restores it, empty plan. Tests run with OpenTofu and Terraform. CI runs the changed plugin folders, and every folder when core changes.

## Consequences

### Positive

- A release is tested on every NetBox and plugin version it declares.

### Negative

- CI time grows with the number of plugins and ranges.

### Risks

- A plugin whose package conflicts with another's cannot share one image; the environment then installs it alone.

## Actions

- `testenv/`: image build and start scripts for Podman and Docker, one profile per NetBox version.
- `make testacc` runs the acceptance tests against the running profile.

## References

- [terraform-plugin-testing](https://developer.hashicorp.com/terraform/plugin/testing)
