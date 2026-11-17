# ADR-0006: Only configured attributes are managed

|                     |            |
| ------------------- | ---------- |
| **Status**          | Accepted   |
| **Date**            | 2026-11-17 |
| **Decision makers** | @ndreno    |

## Context

Plugin models have optional fields with server defaults and fields the plugin computes itself, such as a DNS zone serial. The provider needs a rule for optional attributes left out of a configuration.

## Options considered

### Option 1: Every attribute managed

**Description**: Unset optional attributes are sent as null. The provider owns every field.

| Advantages                             | Disadvantages                                                    |
| -------------------------------------- | ---------------------------------------------------------------- |
| + Removing an attribute clears it      | - Overwrites server defaults and plugin-computed values          |
|                                        | - Needs the default of every field, which the schema rarely states |

### Option 2: Only configured attributes managed

**Description**: Optional attributes are optional and computed. When unset, they take the server value and are not sent. Updates send only changed attributes (PATCH).

| Advantages                                          | Disadvantages                                          |
| --------------------------------------------------- | ------------------------------------------------------ |
| + Server defaults and computed values never diff    | - Removing an attribute leaves its value in NetBox     |
| + Drift detected on every configured attribute      |                                                        |

## Decision

> **Decision**: Option 2 - Only configured attributes managed

**Justification**: The schema does not give defaults reliably, and plugins compute values Terraform must not fight.

For `custom_fields`, the rule applies per key: keys in the configuration are managed and compared; other custom fields are left alone.

## Consequences

### Positive

- Plans stay empty when the plugin updates fields of its own.

### Negative

- Clearing a value takes an explicit empty value (`""`, `[]`), not removing the attribute.

### Risks

- Users may expect removal to clear; the documentation of every resource states the rule.

## Actions

- Schema builder: optional attributes are optional, computed, and keep their state value when unknown.
- Engine: updates send changed attributes only; custom fields are compared per configured key.

## References

- [Terraform Plugin Framework: plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification)
