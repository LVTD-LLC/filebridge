---
title: "Migrate an AI-Agent Dataset to a Business Key"
description: "Move agent-managed rows from a generated ID to a stable business key with mapping, mirrored writes, verification, cutover, and rollback."
published_at: 2026-08-04
updated_at: 2026-08-04
author: Rasul Kireev
keywords:
  - business key database
  - AI agent data migration
  - database primary key migration
  - agent dataset index migration
topics:
  - agent workflows
  - dataset operations
  - row identity
canonical_url: https://rowset.lvtd.dev/blog/migrate-agent-dataset-business-key
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

To migrate an AI-agent dataset from a generated ID to a business key, create a new dataset with
the business key as its index, copy rows through an explicit old-to-new identity map, mirror new
writes during the transition, verify counts and values, then move agents to the new dataset. Keep
the old dataset available until rollback is no longer needed.

Do not change identity in place while agents are still reading and writing the dataset. Row
identity appears in tool calls, retry logic, relationships, logs, exports, and human instructions.
An abrupt key change can make a correct agent update the wrong record or create a duplicate.

Use this five-phase **identity migration contract**:

1. **Map:** prove that every source row has one valid target business key.
2. **Mirror:** create a new dataset and send new changes to both identities.
3. **Verify:** compare row counts, key coverage, relationships, and important values.
4. **Cut over:** point agents at the new dataset under a recorded boundary.
5. **Retire:** archive the old dataset only after the rollback window closes.

## In this guide

- [When to migrate](#when-to-migrate)
- [Define the identity contract](#define-identity-contract)
- [Build the key map](#build-key-map)
- [Create the destination dataset](#create-destination)
- [Backfill and mirror changes](#backfill-and-mirror)
- [Verify the migration](#verify-migration)
- [Cut agents over safely](#cut-over)
- [Close the rollback window](#close-rollback)
- [Business-key migration FAQ](#business-key-migration-faq)

<a id="when-to-migrate"></a>
## When should an agent dataset move to a business key?

Move to a business key when another system, person, or agent already uses a stable identifier for
the same record. Product SKUs, ticket IDs, customer IDs, and source-system record IDs can make
lookups and reconciliation clearer than a generated sequence that exists only inside one dataset.

PostgreSQL defines a primary key as a unique, non-null identifier for rows. It also warns that
adding one changes the data contract by enforcing those constraints ([PostgreSQL 18 constraints
documentation, checked August 2026](https://www.postgresql.org/docs/current/ddl-constraints.html)).
Rowset's index column is not a general SQL primary key, but the same identity test applies: one
non-blank index value must resolve to one row.

Do not migrate merely because a business key looks more descriptive. Keep the generated
`rowset_id` when the proposed key can change, arrive blank, collide, or be reassigned. The
[index-column decision guide](/blog/choose-index-column-agent-rows) provides the full stability
test, and the [Rowset ID versus business-key guide](/blog/rowset-id-vs-business-keys) explains the
tradeoff between internal and external identity.

<a id="define-identity-contract"></a>
## 1. Define the identity contract before copying rows

Write down the migration boundary before creating the destination. A useful contract names:

| Field | Example | Why it matters |
|---|---|---|
| Source dataset | `contacts-v1` | Prevents an agent from copying a similarly named table |
| Source index | `rowset_id` | Preserves the old lookup handle |
| Target dataset | `contacts-v2` | Gives the new identity a separate namespace |
| Target index | `contact_id` | States the business key being adopted |
| Key source | CRM `contact_id` | Establishes who owns identity |
| Snapshot boundary | `2026-08-04T06:30:00Z` | Separates backfill from later writes |
| Validation rules | required, unique, exact string | Makes acceptance deterministic |
| Cutover boundary | migration version or timestamp | Tells agents which dataset is authoritative |
| Rollback deadline | seven days after cutover | Prevents indefinite dual authority |

The business system that issues the key should remain its authority. An agent may transport and
validate `contact_id`; it should not invent missing CRM IDs to make the migration pass.

Google Cloud's Spanner primary-key migration guidance calls out a related risk: applications can
quietly depend on key ordering or key shape, not only uniqueness ([Spanner primary-key migration
overview, checked August 2026](https://docs.cloud.google.com/spanner/docs/primary-keys-overview)).
Audit those assumptions. If an agent treats a larger `rowset_id` as "newer," replace that behavior
with an explicit `created_at` field before cutover.

<a id="build-key-map"></a>
## 2. Build a complete old-to-new key map

Create a private migration dataset indexed by the old identity. Keep the map separate from both
business datasets so it can record exceptions without polluting production rows.

Suggested fields:

```text
source_rowset_id
target_business_key
source_version
mapping_status
conflict_reason
verified_at
```

Populate one mapping row for every source row, then reject these cases:

- a source row has no target key
- two source rows map to the same target key
- one source row maps to more than one target key
- the target key was normalized differently from the source system
- a relationship points at an old index value with no mapped target

Do not trim, lowercase, or otherwise normalize keys unless the identity owner defines that rule.
`CUS-104` and `cus-104` may be the same identifier in one system and different identifiers in
another. Record the rule instead of letting an agent infer it.

The mapping dataset is also the rollback bridge. If an operator reports a problem against the new
`contact_id`, the agent can resolve the old `rowset_id`, inspect both rows, and explain the
difference without fuzzy search.

<a id="create-destination"></a>
## 3. Create a new dataset with the business key

In Rowset, create a new dataset rather than trying to rename or drop the active generated index.
Generated index values are managed by Rowset, generated index columns cannot be renamed, and an
index column cannot be dropped. A new dataset makes the identity change reviewable and preserves
a clean rollback path.

Create the destination with the target key present in `headers` and selected as `index_column`:

```json
{
  "name": "Contacts v2",
  "description": "Agent-managed contacts keyed by the CRM contact ID",
  "instructions": "Use contact_id for exact lookup. Do not create IDs. Verify CRM evidence before writes.",
  "headers": ["contact_id", "name", "email", "status", "source_updated_at"],
  "index_column": "contact_id",
  "column_types": {
    "contact_id": {
      "type": "text",
      "description": "Stable contact identifier issued by the CRM"
    },
    "email": "email",
    "status": {
      "type": "choice",
      "choices": ["active", "inactive", "unknown"]
    },
    "source_updated_at": "datetime"
  }
}
```

The live [Dataset API documentation](/docs/dataset-api) confirms that Rowset accepts an explicit
index at creation and adds `rowset_id` only when `index_column` is omitted. Put field meaning and
workflow rules in the schema, instructions, and metadata so a future agent does not have to infer
them from the migration notes. See [designing a dataset schema](/docs/design-schema) for semantic
column types and descriptions.

<a id="backfill-and-mirror"></a>
## 4. Backfill the snapshot, then mirror new changes

Record a snapshot boundary and copy only rows that have an accepted mapping. For each row:

1. Read the source row by `rowset_id`.
2. Read its accepted target key from the mapping dataset.
3. Transform only fields covered by the migration contract.
4. Create the destination row with the target business key.
5. Read it back by the target key.
6. Compare the stored values with the migration payload.
7. Mark the mapping row verified or record a typed exception.

Rowset dataset creation accepts up to 1,000 initial rows. For larger migrations, page through the
source and write bounded batches after creating the destination. The [row operations guide]
(/docs/work-with-rows) documents exact by-index reads and updates for MCP and REST.

After the snapshot starts, route every new write through one migration worker. The worker writes
the current authoritative dataset first, applies the equivalent change to the other dataset, and
records both outcomes. If either response is uncertain, read both rows before retrying. The
[idempotent agent-update pattern](/blog/idempotent-ai-agent-updates) covers this reconciliation
path in detail.

Do not ask independent agents to dual-write from prompt instructions. A prompt cannot provide
atomicity, ordering, or recovery after a timeout. One deterministic worker should own mirroring.

<a id="verify-migration"></a>
## 5. Verify identity, values, and relationships

A matching row count is necessary, not sufficient. Run four checks:

1. **Identity coverage:** every accepted source row has one destination key, and every destination
   row points back to one mapping record.
2. **Value parity:** compare fields that should remain unchanged, using exact values or a canonical
   hash over an agreed field list.
3. **Relationship coverage:** translate old index values and prove that every required target
   exists before rebuilding links.
4. **Change convergence:** every mirrored write after the snapshot has the same final values in
   both datasets or a resolved exception.

Keep counts by status rather than one global pass flag:

```text
source_rows=842
accepted_mappings=839
blocked_mappings=3
destination_rows=839
value_matches=839
relationship_targets_missing=0
mirror_exceptions_open=0
```

These numbers are illustrative, not Rowset customer metrics. The important property is the
equation: destination rows should equal accepted mappings, while blocked rows remain visible and
do not silently disappear.

<a id="cut-over"></a>
## 6. Cut agents over under a recorded boundary

Cut over only when the verification report passes and blocked mappings have an explicit owner.
Update the agent setup, dataset key, instructions, relationship targets, scheduled jobs, and any
saved API configuration together. Do not rely on the dataset name alone; agents should use the
new dataset key and `contact_id` lookup contract.

At the cutover boundary:

1. Pause old-dataset writers.
2. Drain and verify the mirror queue.
3. Record the final old-dataset version or timestamp.
4. Switch reads and writes to the new dataset key.
5. Run a canary read, create-or-update, and read-back by business key.
6. Resume workers only after the canary passes.

Keep the old dataset read-only during the rollback window. If the canary or later reconciliation
fails, pause writers, resolve new keys through the mapping dataset, and return to the old dataset
without reconstructing identity from names or search results.

<a id="close-rollback"></a>
## 7. Retire the old identity without deleting evidence

Close the migration after the agreed window and a final convergence check. Archive the old dataset
rather than deleting it immediately. Preserve the migration contract, key map, verification
report, cutover event, and exception decisions according to the workflow's retention needs.

Update durable instructions so future agents know that the business key is authoritative. Remove
the dual-write path, revoke obsolete access, and test that no scheduled worker still references
the old dataset key. Migration code that remains active becomes a second, accidental source of
truth.

Rowset is useful here because the migration artifacts can remain private datasets beside the
operational records, accessible through [hosted MCP](/docs/connect-mcp) or REST. Rowset does not
turn a generated index into a business key in place; it gives trusted agents a structured surface
for the mapping, destination, verification, and cutover records. Review [Rowset pricing]
(/pricing) if you want to run the pattern with hosted datasets.

<a id="business-key-migration-faq"></a>
## Business-key migration FAQ

### Can I rename `rowset_id` to my business key?

No. A Rowset-generated index is managed metadata and cannot be renamed. Create a new dataset with
the business key selected as `index_column`, copy rows through a verified mapping, and cut agents
over after validation. This preserves rollback and prevents a live identity contract from changing
under active agents.

### Should an AI agent generate missing business keys?

Only when the agent is explicitly the authorized key issuer and the generation rule is
deterministic. In most migrations, the upstream CRM, catalog, ticket system, or application owns
the key. Missing keys should enter an exception queue rather than being guessed from names,
emails, or row positions.

### How long should dual writes continue?

Continue until the backfill is complete, post-snapshot changes have converged, and a canary proves
the new read and write path. Set a fixed end condition and rollback window. Indefinite dual writes
create two authorities and make later discrepancies harder to explain.

### Is a matching row count enough to verify the migration?

No. Verify one-to-one key coverage, important field values, translated relationships, and every
change written after the snapshot. A matching count can still hide duplicated keys, missing rows,
stale values, or links that point at the old identity.
