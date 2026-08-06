---
title: "Composite Primary Keys for AI-Agent Datasets"
description: "Model multi-field row identity for AI agents with a deterministic composite index, explicit component rules, and safe Rowset lookups."
published_at: 2026-08-06
updated_at: 2026-08-06
author: Rasul Kireev
keywords:
  - composite primary key
  - AI agent dataset identity
  - composite business key
  - deterministic row index
topics:
  - agent workflows
  - dataset operations
  - row identity
canonical_url: https://rowset.lvtd.dev/blog/composite-primary-key-ai-agents
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

A composite primary key identifies one row with a fixed combination of two or more fields. For an
AI-agent dataset, use composite identity only when no component is unique by itself and the whole
tuple is stable. If the storage API accepts one index column, encode that tuple deterministically
and keep the original components as separate fields.

An inventory record may be unique only within a workspace, so `(workspace_id, sku)` is the real
identity. A translation may be unique only within a document and locale, so
`(document_id, locale)` identifies it. Flattening either tuple into an improvised string can make
agent retries unsafe unless every writer follows the same construction rule.

This guide uses a five-part **SCOPE identity contract**:

1. **Scope:** name the boundary inside which the record is unique.
2. **Components:** fix the ordered fields that form identity.
3. **Ownership:** record which system owns each component.
4. **Percent-encode:** turn the tuple into one reversible, delimiter-safe index.
5. **Evolution:** version the encoding and migrate instead of silently changing it.

## In this guide

- [What is a composite primary key?](#what-is-composite-primary-key)
- [When does an AI-agent dataset need one?](#when-agents-need-composite-identity)
- [The SCOPE identity contract](#scope-identity-contract)
- [Build a deterministic composite index](#build-composite-index)
- [Create the Rowset dataset](#create-rowset-dataset)
- [Handle retries and relationships](#retries-and-relationships)
- [When to use a generated ID instead](#when-to-use-generated-id)
- [Composite primary key FAQ](#composite-primary-key-faq)

<a id="what-is-composite-primary-key"></a>
## What is a composite primary key?

A composite primary key is a primary key made from multiple columns whose combined values uniquely
identify a row. Each component alone may repeat; the complete tuple must be unique and non-null.

PostgreSQL documents primary keys as a column or group of columns whose values uniquely identify
rows, and it supports declarations such as `PRIMARY KEY (workspace_id, sku)`
([PostgreSQL 18 constraints documentation, checked August
2026](https://www.postgresql.org/docs/current/ddl-constraints.html)). In a relational database,
the database enforces the tuple directly.

Framework support is a separate concern. Django added `CompositePrimaryKey` in version 5.2, but
its current documentation still lists migration and relationship limitations
([Django composite primary key documentation, checked August
2026](https://docs.djangoproject.com/en/6.0/topics/composite-primary-key/)). That distinction
matters for agents: a valid database design can still be awkward at the API or tool boundary.

Rowset exposes one `index_column` for exact row lookup. It does not claim to provide a native
multi-column SQL primary key. When your source identity is a tuple, the practical Rowset pattern is
to retain the component columns and derive one deterministic text index from them.

<a id="when-agents-need-composite-identity"></a>
## When does an AI-agent dataset need composite identity?

Use composite identity when the workflow naturally names a record with more than one stable value.
Common examples include:

| Workflow | Identity tuple | Why one field is insufficient |
|---|---|---|
| Multi-workspace catalog | `(workspace_id, sku)` | The same SKU can exist in different workspaces. |
| Localized content | `(content_id, locale)` | One content item has several locale-specific rows. |
| Daily measurement | `(device_id, observed_date)` | A device has one accepted measurement per day. |
| Membership record | `(team_id, person_id)` | A person can belong to several teams. |
| Source synchronization | `(source_system, source_id)` | Different systems can issue the same local ID. |

Do not add components merely because they are available. If `event_id` is already globally unique,
then `(workspace_id, event_id)` usually gives agents a wider, more fragile handle without improving
identity. The [index-column decision guide](/blog/choose-index-column-agent-rows) applies the same
required, unique, stable, and recognizable tests to single-field keys.

Also separate identity from state. `(task_id, status)` is a poor composite key because `status` is
supposed to change. `(customer_id, email)` is risky when email can be corrected. Every component
becomes part of the lookup contract, so changing one changes the row's apparent identity.

<a id="scope-identity-contract"></a>
## Use the SCOPE identity contract

The SCOPE contract turns a plausible tuple into a specification an agent can follow after its
current conversation ends.

### 1. Scope: state the uniqueness boundary

Write the sentence that explains where the record is unique:

```text
A catalog item is unique inside one workspace by the pair
(workspace_id, sku).
```

If you cannot name the boundary, you probably have not found the identity yet. `sku` may be unique
per supplier rather than per workspace. A locale code may need a document version. Resolve that
ambiguity before creating rows.

### 2. Components: fix names, order, and validation

List the components in one order and never let callers rearrange them. For example:

```text
components = [workspace_id, sku]
workspace_id = required, exact lowercase slug
sku = required, case-sensitive supplier value
```

`(workspace_id, sku)` and `(sku, workspace_id)` contain the same values but must not produce two
indexes. The contract also decides whether whitespace, case, Unicode, and leading zeroes are
meaningful. Do not let each agent normalize those details independently.

### 3. Ownership: identify who may issue or change each value

Name the authority for every component. Rowset may store `workspace_id`, but the workspace service
owns it. An agent may transport a supplier SKU, but the supplier catalog defines its exact value.

Ownership prevents a well-meaning agent from inventing `unknown-workspace`, trimming a significant
leading zero, or lowercasing an identifier simply to make validation pass. Missing identity should
become a typed exception or review item, not fabricated data.

### 4. Percent-encode: make one reversible index

When an API accepts one text index, encode each component separately and join the encoded values
with a reserved delimiter. RFC 3986 defines percent-encoding for representing data characters that
would otherwise conflict with URI syntax
([RFC 3986, January 2005](https://www.rfc-editor.org/rfc/rfc3986)).

For this contract, encode the UTF-8 form of every component and join them with `|`. Because a raw
pipe inside a component becomes `%7C`, splitting the finished index on `|` remains unambiguous.

### 5. Evolution: version the format

Prefix the result with an encoding version such as `ck1`. The prefix is not a schema version for
the entire dataset. It tells a future reader exactly how to decode this identity string.

If component order or normalization changes, create `ck2` values in a new dataset and use an
explicit migration. Do not reinterpret existing `ck1` values in place. The
[business-key migration guide](/blog/migrate-agent-dataset-business-key) covers mapping, mirrored
writes, verification, cutover, and rollback.

<a id="build-composite-index"></a>
## How do you build a deterministic composite index?

Build the index in one shared function, reject blank components, and test collision-shaped inputs.
This Python example uses only the standard library:

```python
from urllib.parse import quote


def catalog_index(workspace_id: str, sku: str) -> str:
    components = (workspace_id, sku)
    if any(value == "" for value in components):
        raise ValueError("Composite identity components must be non-blank")
    encoded = (quote(value, safe="") for value in components)
    return "ck1|" + "|".join(encoded)
```

The same values always produce the same lookup handle:

| `workspace_id` | `sku` | `composite_id` |
|---|---|---|
| `acme` | `SKU-104` | `ck1|acme|SKU-104` |
| `acme` | `US|42` | `ck1|acme|US%7C42` |
| `acme east` | `A/B` | `ck1|acme%20east|A%2FB` |

Avoid raw concatenation such as `workspace_id + "-" + sku`. The tuples `("a-b", "c")` and
`("a", "b-c")` both flatten to `a-b-c`. JSON stringification is safer than raw concatenation,
but different serializers can vary in whitespace and escaping unless you define a canonical form.
RFC 8785 exists because repeatable hashing and signing require invariant JSON serialization
([JSON Canonicalization Scheme, June 2020](https://www.rfc-editor.org/rfc/rfc8785)). For a short
tuple of text identifiers, per-component encoding is easier to inspect and reproduce.

Test the encoder with delimiter characters, percent signs, spaces, non-ASCII text, case variants,
and leading zeroes. Test that approved normalization is applied before encoding and that every
unapproved transformation is rejected.

<a id="create-rowset-dataset"></a>
## How do you store composite identity in Rowset?

Keep both the derived index and its source components. The derived field powers exact lookup; the
component fields keep exports, reviews, and relationships understandable.

```json
{
  "name": "Workspace catalog",
  "description": "Catalog records unique by workspace and supplier SKU",
  "instructions": "Use composite_id for exact lookup. Build it as ck1|percent_encode(workspace_id)|percent_encode(sku). Preserve component case. Never invent missing identity values.",
  "headers": ["composite_id", "workspace_id", "sku", "name", "status"],
  "index_column": "composite_id",
  "column_types": {
    "composite_id": {
      "type": "text",
      "description": "Versioned deterministic index derived from workspace_id and sku"
    },
    "workspace_id": {
      "type": "text",
      "description": "Exact workspace slug issued by the workspace service"
    },
    "sku": {
      "type": "text",
      "description": "Case-sensitive supplier SKU"
    },
    "status": {
      "type": "choice",
      "choices": ["active", "inactive"]
    }
  },
  "metadata": {
    "identity": {
      "version": "ck1",
      "components": ["workspace_id", "sku"],
      "separator": "|",
      "encoding": "RFC 3986 percent-encoding over UTF-8"
    }
  }
}
```

Rowset's [Dataset API](/docs/dataset-api) accepts one explicit `index_column` at creation. The
[schema design guide](/docs/design-schema) explains how column descriptions, dataset instructions,
and JSON metadata give future agents durable field meaning and workflow rules.

Generate `composite_id` before the write. After creating or updating a row, read it back by that
index and compare `workspace_id` and `sku` with the payload. Treat a mismatch as a contract failure,
not as permission to overwrite the components.

<a id="retries-and-relationships"></a>
## How does composite identity affect retries and relationships?

A deterministic composite index gives every retry the same target. If a create response is lost,
the agent can rebuild `ck1|acme|SKU-104`, look up the row, compare the stored values, and then decide
whether a new write is necessary. The [idempotent update guide](/blog/idempotent-ai-agent-updates)
shows the full inspect, compare, mutate, and verify sequence.

Relationships need the same discipline. Rowset relationships store a target dataset's index value,
so a source row should keep the complete `composite_id`, not one component or a display name. The
[relationship documentation](/docs/link-datasets) covers enforcement and resolution through MCP
and REST.

If another system already supports multi-column foreign keys, keep using its native tuple there.
The encoded Rowset index is an adapter for a single-index tool boundary, not a reason to flatten the
source database's relational design.

<a id="when-to-use-generated-id"></a>
## When should you use a generated ID instead?

Use a generated `rowset_id` or a stable upstream surrogate ID when the proposed tuple is mutable,
partially unknown, excessively long, or inconsistently normalized across systems. Keep a separate
uniqueness rule over the business fields in the source system when it can enforce one.

Choose generated identity when:

- one or more components can change during normal work
- the identity authority cannot define canonical case or whitespace rules
- agents often receive only part of the tuple
- downstream tools cannot reproduce the encoding exactly
- the source already provides a globally unique immutable ID

The [Rowset ID versus business-key guide](/blog/rowset-id-vs-business-keys) explains that tradeoff.
Rowset can generate `rowset_id` when `index_column` is omitted, giving the agent a stable internal
handle without pretending uncertain business fields form safe identity.

Use native composite primary keys in a relational database when the database should enforce the
tuple directly. Use a deterministic composite index in Rowset when trusted agents need one exact
lookup value across MCP or REST. Those are compatible choices at different boundaries.

Review [Rowset pricing](/pricing) if you want to operate the pattern with private hosted datasets,
or start with the [Rowset quickstart](/docs/quickstart) to create and inspect a dataset through an
agent.

<a id="composite-primary-key-faq"></a>
## Composite primary key FAQ

### Can a primary key contain multiple columns?

Yes. PostgreSQL and other relational databases can enforce one primary key across a group of
columns. The combined tuple must uniquely identify each row, and primary-key components cannot be
null. Framework and API support may impose additional limitations even when the database supports
the design.

### Is a composite primary key better than a surrogate key?

Neither is universally better. Use composite identity when a stable real-world tuple is the record's
actual identity and every caller can provide it. Use a surrogate key when the tuple is mutable,
wide, optional, or difficult to carry through APIs and relationships. You may still enforce a
separate unique constraint over business fields in the source database.

### Does Rowset support a native multi-column primary key?

Rowset exposes one index column for exact by-index operations. To preserve multi-field identity,
keep the component fields and derive one deterministic, versioned text index. This is an agent
lookup contract, not a native SQL composite-primary-key declaration.

### Why not join composite-key fields with a hyphen?

A raw separator can appear inside a component and create collisions. Encode every component first,
use a separator excluded from the encoded component alphabet, fix component order, and version the
format. Then agents in different sessions can reproduce the same lookup value safely.
