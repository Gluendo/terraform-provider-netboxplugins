# ADR-0004: Coexistence with the core NetBox provider

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

Plugin objects reference core objects: IP addresses, prefixes, tenants, tags. Users manage those with `e-breuninger/netbox`.

## Options considered

### Option 1: Include or fork the core provider's code

**Description**: Expose core resources from this provider too.

| Advantages                      | Disadvantages                                                  |
| ------------------------------- | -------------------------------------------------------------- |
| + One provider for everything   | - Duplicates a large, well-maintained provider                 |
|                                 | - Ties this provider to the core provider's NetBox version support |

### Option 2: Independent provider, same conventions, references by ID

**Description**: Only plugin models. Configuration attributes and environment variables match the core provider; core objects are referenced by numeric ID.

| Advantages                                  | Disadvantages                                         |
| ------------------------------------------- | ----------------------------------------------------- |
| + One set of variables for both providers   | - Two provider blocks in a configuration              |
| + No code or release coupling               |                                                       |

## Decision

> **Decision**: Option 2 - Independent provider, same conventions, references by ID

**Justification**: The core provider covers core models well; this provider adds what it does not cover.

| Attribute              | Environment variable          |
| ---------------------- | ----------------------------- |
| `server_url`           | `NETBOX_SERVER_URL`           |
| `api_token`            | `NETBOX_API_TOKEN`            |
| `allow_insecure_https` | `NETBOX_ALLOW_INSECURE_HTTPS` |
| `request_timeout`      | `NETBOX_REQUEST_TIMEOUT`      |
| `headers`              |                               |

References to core objects take their ID (`netbox_ip_address.web.id`). Tags take names, as in the core provider.

## Consequences

### Positive

- A configuration mixes both providers with no glue.

### Negative

- Users declare two providers.

### Risks

- If the core provider renames its environment variables, users must set both; tests use the core provider to catch it.

## Actions

- Acceptance tests create core objects with `e-breuninger/netbox` and reference them from plugin resources.

## References

- [e-breuninger/terraform-provider-netbox](https://github.com/e-breuninger/terraform-provider-netbox)
