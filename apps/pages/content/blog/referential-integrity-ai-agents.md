---
title: "Referential Integrity for AI-Agent Datasets"
description: "Use referential integrity to stop AI agents from creating orphan records, broken links, and unsafe deletes across related datasets."
published_at: 2026-08-11
updated_at: 2026-08-11
author: Rasul Kireev
keywords:
  - referential integrity
  - referential integrity constraint
  - orphan records
  - foreign key constraint
topics:
  - agent workflows
  - dataset operations
  - relationships
canonical_url: https://rowset.lvtd.dev/blog/referential-integrity-ai-agents
image: /static/vendors/images/logo.png
image_alt: Rowset logo
robots: index, follow
---

Referential integrity means every stored reference points to a valid target record, or is
deliberately blank. In an AI-agent workflow, protect it before a child write, after an uncertain
response, before renaming or deleting a target, and during periodic orphan checks. The agent should
never repair a broken link by guessing.

For example, if `Messages.person_id` points to `People.person_id`, every non-blank person ID in a
message must identify a real person row. A missing target creates an orphan record: the message
claims a relationship that the system cannot resolve.

This guide uses a five-step **VALID integrity contract**:

1. **Verify** the target exists.
2. **Add** the parent first when creation is authorized.
3. **Link** the exact stable index value.
4. **Inspect** the stored child and resolved target after the write.
5. **Delete** or rename targets deliberately, with dependent rows handled first.

## In this guide

- [What is referential integrity?](#what-is-referential-integrity)
- [Why is it different for AI agents?](#referential-integrity-ai-agents)
- [The VALID integrity contract](#valid-integrity-contract)
- [How Rowset enforces relationships](#rowset-referential-integrity)
- [How to enable enforcement on existing data](#enable-integrity-existing-data)
- [Which delete behavior should you choose?](#referential-actions)
- [How to recover from a violation](#repair-integrity-violation)
- [Referential integrity FAQ](#referential-integrity-faq)

<a id="what-is-referential-integrity"></a>
## What is referential integrity?

Referential integrity is the rule that every reference between records remains valid. In a
relational database, a foreign key in a child table must match a primary or unique key in a parent
table. PostgreSQL defines a foreign-key constraint as requiring values in one column or group of
columns to match values in a row of another table
([PostgreSQL constraints, checked August 2026](https://www.postgresql.org/docs/current/ddl-constraints.html)).

Three mutations can break that rule:

- inserting or updating a child with a target ID that does not exist
- deleting a parent that still has dependent children
- changing a parent's referenced key without updating its children

The terms describe roles, not importance. A People row is the parent of a Message row when the
message stores its `person_id`. The same People row can be a child in another relationship, such
as `People.account_id -> Accounts.account_id`.

Referential integrity is narrower than data integrity. Data integrity also covers valid types,
ranges, required fields, uniqueness, and business rules. Referential integrity deals specifically
with whether links between records point somewhere real.

The existing guide to [modeling relationships between agent-managed datasets](/blog/relationship-modeling-agent-datasets)
helps decide when two entities deserve separate datasets and which stable index should connect
them. This guide starts after that design decision. Its job is to keep the chosen link valid while
an agent changes data.

<a id="referential-integrity-ai-agents"></a>
## Why does referential integrity need an agent workflow?

A database constraint can reject an invalid write, but an agent still has to decide what to do
next. That decision is where unsafe repairs appear.

Suppose an agent tries to create a message for `P-404` and receives "target row not found." Several
responses are possible:

- stop and report the missing person
- search for the correct existing person
- create a new person, if the user and dataset instructions authorize it
- leave the relationship blank and route the message to review, if blanks are allowed
- invent a plausible person ID and retry

Only the last response is always wrong. The other choices depend on workflow authority and the
meaning of the missing target. A constraint detects the broken link; durable instructions define
the safe recovery path.

Agent tool calls also create an ordering problem. Creating a person and creating a message may be
two separate MCP or REST operations. The first call can succeed while its response is lost, or the
second can fail after the parent exists. Treating the pair as if it were one atomic operation can
produce duplicates on retry.

Use a [stable index and idempotent update pattern](/blog/idempotent-ai-agent-updates): read by the
business key after an uncertain response, reuse the confirmed target, then write the child. Do not
mint a second parent merely because the agent did not receive the first response.

<a id="valid-integrity-contract"></a>
## Use the VALID integrity contract

VALID turns referential integrity from a database error into an agent operating procedure.

### 1. Verify the target

Resolve the relationship before writing the child. Search when the target dataset is unknown, then
use an exact index lookup once you know the dataset and key.

For `Messages.person_id = P-17`, verify that `People` contains `P-17`. A fuzzy match on a display
name is evidence for review, not permission to substitute a different ID. If several source systems
use different IDs, translate them through an approved
[crosswalk table](/blog/crosswalk-table-ai-agents) first.

### 2. Add the parent first

Create the parent only when the workflow authorizes creation and search has ruled out an existing
record. Give it a stable index before any child refers to it.

Parent-first ordering makes the invariant easy to state: a child write never introduces a target
that the agent has not already read or created. It also keeps a failure on the child side from
leaving a broken reference.

### 3. Link the exact stable index

Store the target's index value, not a label that happens to look unique. Names, titles, and mutable
emails are weak relationship handles. Values such as `person_id`, `sku`, `ticket_id`, and
`content_id` make exact lookup and enforcement predictable.

If the target is identified by several fields, encode the full identity deliberately. The
[composite-primary-key guide](/blog/composite-primary-key-ai-agents) shows how to carry a scoped
tuple through a single-index agent interface without ambiguous concatenation.

### 4. Inspect the stored result

Read the child after a successful or uncertain write, then resolve its relationship. Confirm two
facts separately:

1. the child contains the intended target index value
2. that value resolves to the intended target row

This catches serialization mistakes, trimmed or transformed values, writes to the wrong dataset,
and timeouts where the mutation succeeded but the response did not arrive.

### 5. Delete or rename deliberately

Before deleting a parent or changing its index, list or count incoming references. Decide whether
the children should be deleted, reassigned, detached, or preserved with the operation blocked.

For agent-managed operational data, blocking is the safest default. Automatic cascading deletes
can turn one mistaken tool call into the removal of an entire dependent record set. Use a cascade
only when the child has no meaning outside the parent and the destructive scope is explicit.

<a id="rowset-referential-integrity"></a>
## How does Rowset enforce referential integrity?

Rowset relationships connect a source dataset column to the index column of a target dataset. A
relationship can say `Messages.person_id -> People.person_id`, while both datasets remain available
through authenticated [MCP tools](/docs/connect-mcp) and the [Dataset API](/docs/dataset-api).

With relationship enforcement enabled, Rowset:

- accepts a non-blank source value only when the target index exists
- allows a blank value for a relationship that is not known yet
- rejects enabling enforcement while existing non-blank values lack targets
- blocks deletion of a referenced target row
- blocks changing a target index while enforced source rows still reference it

These rules protect target existence and identity. They do not decide whether a missing person
should be created, whether an optional link may remain blank, or who may approve a destructive
cleanup. Put those decisions in dataset instructions and review policy.

Inspect the dataset with `get_dataset` before row work so the agent sees outgoing and incoming
relationship summaries. Use `resolve_dataset_relationship` when you need the target row behind a
source value. The [link-datasets reference](/docs/link-datasets) lists the equivalent REST routes.

An instruction block can make the repair boundary explicit:

```text
Messages.person_id points to People.person_id.
Resolve person_id before creating or updating a message.
If the target is missing, search People by exact external identifiers.
Do not create a new person or replace person_id from a name match without approval.
After a write, read the message and resolve the relationship.
Do not delete or rename a referenced People row until dependent messages are reviewed.
```

<a id="enable-integrity-existing-data"></a>
## How do you enable enforcement on existing data?

Audit first. Turning on enforcement without checking existing values can fail immediately or hide
how many exceptions need a real decision.

Use this sequence:

1. List distinct non-blank values in the proposed source column.
2. Compare them with exact index values in the target dataset.
3. Put missing values in a review queue; do not auto-create targets from display labels.
4. Correct confirmed mistakes, create authorized targets, or blank explicitly optional links.
5. Create the enforced relationship only after every remaining non-blank value resolves.
6. Run a final sample of relationship resolutions and record the audit date.

Keep source evidence for every correction. If `P-071` becomes `P-17`, record why those values refer
to the same person instead of silently rewriting the child. The
[AI-agent audit trail guide](/blog/ai-agent-audit-trail) provides a durable mutation record for the
actor, before/after values, reason, evidence, and result.

When messy source material must land before matching is complete, stage it outside the enforced
dataset or start with enforcement disabled and an explicit review status. An unenforced
relationship is a navigation hint, not proof that every target exists.

<a id="referential-actions"></a>
## Which referential action should an agent workflow use?

SQL databases provide several actions for target updates and deletes. PostgreSQL and SQLite both
document `NO ACTION`, `RESTRICT`, `CASCADE`, `SET NULL`, and `SET DEFAULT` for foreign keys
([PostgreSQL](https://www.postgresql.org/docs/current/ddl-constraints.html),
[SQLite](https://www.sqlite.org/foreignkeys.html), checked August 2026).

| Action | What happens | Agent-workflow guidance |
|---|---|---|
| Restrict / no action | The parent change is blocked while children depend on it. | Best default for independently valuable operational records. |
| Cascade | Parent deletion or key update propagates to children. | Use only when children cannot exist independently and the full destructive scope is approved. |
| Set null | Child references become blank. | Useful for optional links when the child remains meaningful and a review queue catches detachment. |
| Set default | Child references receive a configured default. | Use only when the default is a real valid target with clear business meaning. |

PostgreSQL distinguishes `RESTRICT` from deferrable `NO ACTION` and advises choosing the delete
behavior from the meaning of the related objects. If child rows are components that cannot exist
independently, cascading may fit. If parent and child represent independent objects, blocking the
delete is usually more appropriate.

Rowset's enforced relationships follow the conservative operational choice: referenced target
deletion and index changes are blocked. If you intend to remove a target, update or delete its
dependent rows explicitly first. This keeps each mutation visible to the agent and reviewable by a
human.

<a id="repair-integrity-violation"></a>
## What happens if referential integrity is violated?

An enforced system rejects the invalid mutation. An unenforced system can retain an orphan that
fails to resolve later. In either case, use the evidence to repair the relationship. Clearing the
error message alone is incomplete.

Use this recovery order:

1. Preserve the rejected payload or orphan row as evidence.
2. Inspect the source dataset, relationship definition, and target index column.
3. Search for the intended target using stable source identifiers.
4. Classify the cause: typo, wrong namespace, missing parent, retired target, or unauthorized
   creation.
5. Propose one explicit repair and require review when identity or deletion is uncertain.
6. Apply the change with an idempotent write.
7. Read the child and resolve the target again.

Do not make the error disappear by disabling enforcement permanently. If temporary unmatched rows
are legitimate, model that state: allow blank links, use a staging dataset, or keep a review status
that distinguishes `unmatched` from `approved`.

## A practical checklist

Before an AI agent writes related records, confirm:

- the parent and child datasets have separate, useful jobs
- the target index is stable, unique, and visible to the agent
- the source column stores that exact index value
- dataset instructions define when target creation is authorized
- uncertain tool responses trigger read-back, not blind retry
- existing source values have been audited before enforcement
- target deletes and index changes are blocked or explicitly reviewed
- repairs preserve evidence and finish with relationship resolution

When these checks pass, referential integrity gives the agent a clear stop condition and gives
reviewers a durable contract for every linked write. You can test the pattern with a private
dataset through the [Rowset quickstart](/docs/quickstart) and keep the hosted product after the
trial on [Rowset Pro](/pricing).

<a id="referential-integrity-faq"></a>
## Referential integrity FAQ

### What is meant by referential integrity?

Referential integrity means every non-blank reference in a child record matches a valid key in its
parent record. It prevents inserts, updates, key changes, or deletes from leaving links that point
to missing data.

### What is the difference between data integrity and referential integrity?

Data integrity covers the overall correctness of stored data, including types, required values,
uniqueness, ranges, and business rules. Referential integrity is one part of data integrity. It
focuses on keeping references between records valid.

### Should an AI agent create a missing parent automatically?

Only when the user or dataset instructions authorize creation and an exact search confirms the
parent does not already exist. A missing target may be a typo, wrong ID namespace, delayed import,
or duplicate. The agent should not turn every lookup failure into a new record.

### Are blank relationship values referential-integrity violations?

Not necessarily. Optional relationships may be blank by design. A required relationship should be
enforced with both a valid target check and a required-value rule; target validation alone does not
make a blank field invalid.

### Is referential integrity the same as a foreign key?

No. Referential integrity is the property that references remain valid. A foreign-key constraint
is a database mechanism for enforcing that property. Rowset relationships provide similar
target-existence protection at the dataset and agent-tool layer.
